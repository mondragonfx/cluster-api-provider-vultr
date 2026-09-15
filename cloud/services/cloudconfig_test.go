package services

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

const testCmd = "ufw disable"

func TestPrependCloudConfigRunCmds(t *testing.T) {
	tests := []struct {
		name     string
		userData string
		cmds     []string
		wantErr  bool
		check    func(t *testing.T, out string)
	}{
		{
			name:     "prepends to existing runcmd",
			userData: "#cloud-config\nruncmd:\n  - kubeadm join --config /run/kubeadm/kubeadm.yaml\nwrite_files:\n- path: /etc/foo\n  content: |\n    line one\n    line two\n",
			cmds:     []string{testCmd},
			check: func(t *testing.T, out string) {
				doc := parseCloudConfig(t, out)
				runcmd := doc["runcmd"].([]interface{})
				if len(runcmd) != 2 || runcmd[0] != testCmd || runcmd[1] != "kubeadm join --config /run/kubeadm/kubeadm.yaml" {
					t.Errorf("unexpected runcmd: %v", runcmd)
				}
				files := doc["write_files"].([]interface{})
				content := files[0].(map[string]interface{})["content"].(string)
				if content != "line one\nline two\n" {
					t.Errorf("multi-line content not preserved: %q", content)
				}
			},
		},
		{
			name:     "creates runcmd when absent",
			userData: "#cloud-config\nwrite_files:\n- path: /etc/foo\n  content: bar\n",
			cmds:     []string{testCmd},
			check: func(t *testing.T, out string) {
				doc := parseCloudConfig(t, out)
				runcmd, ok := doc["runcmd"].([]interface{})
				if !ok || len(runcmd) != 1 || runcmd[0] != testCmd {
					t.Errorf("runcmd not created: %v", doc["runcmd"])
				}
				if _, ok := doc["write_files"]; !ok {
					t.Error("write_files lost")
				}
			},
		},
		{
			name:     "keeps list-form commands",
			userData: "#cloud-config\nruncmd:\n  - [sh, -c, 'echo hi']\n",
			cmds:     []string{testCmd},
			check: func(t *testing.T, out string) {
				runcmd := parseCloudConfig(t, out)["runcmd"].([]interface{})
				if len(runcmd) != 2 {
					t.Fatalf("unexpected runcmd length: %v", runcmd)
				}
				if _, isList := runcmd[1].([]interface{}); !isList {
					t.Errorf("list-form command not preserved: %v", runcmd[0])
				}
			},
		},
		{
			name:     "keeps the cloud-init jinja template header (Cluster API bootstrap data)",
			userData: "## template: jinja\n#cloud-config\nruncmd:\n  - kubeadm join\nwrite_files:\n- path: /a\n  content: \"vultr://{{ ds.meta_data['instance_id'] }}\"\n",
			cmds:     []string{testCmd},
			check: func(t *testing.T, out string) {
				if !strings.HasPrefix(out, "## template: jinja\n#cloud-config\n") {
					t.Fatalf("jinja header not preserved: %q", out)
				}
				doc := parseCloudConfig(t, strings.TrimPrefix(out, "## template: jinja\n"))
				runcmd := doc["runcmd"].([]interface{})
				if len(runcmd) != 2 || runcmd[0] != testCmd {
					t.Errorf("unexpected runcmd: %v", runcmd)
				}
				content := doc["write_files"].([]interface{})[0].(map[string]interface{})["content"].(string)
				if content != "vultr://{{ ds.meta_data['instance_id'] }}" {
					t.Errorf("jinja expression altered: %q", content)
				}
			},
		},
		{
			name:     "non cloud-config passes through unchanged",
			userData: "#!/bin/bash\necho hello\n",
			cmds:     []string{testCmd},
			check: func(t *testing.T, out string) {
				if out != "#!/bin/bash\necho hello\n" {
					t.Errorf("script was modified: %q", out)
				}
			},
		},
		{
			name:     "no commands is a no-op",
			userData: "#cloud-config\nruncmd:\n  - a\n",
			cmds:     nil,
			check: func(t *testing.T, out string) {
				if out != "#cloud-config\nruncmd:\n  - a\n" {
					t.Errorf("document was modified: %q", out)
				}
			},
		},
		{
			name:     "runcmd that is not a list is an error",
			userData: "#cloud-config\nruncmd: not-a-list\n",
			cmds:     []string{testCmd},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := PrependCloudConfigRunCmds(tt.userData, tt.cmds)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(out, "#cloud-config") && strings.HasPrefix(tt.userData, "#cloud-config") {
				t.Errorf("header lost: %q", out)
			}
			tt.check(t, out)
		})
	}
}

func parseCloudConfig(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	if !strings.HasPrefix(s, "#cloud-config\n") {
		t.Fatalf("missing #cloud-config header: %q", s)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, s)
	}
	return doc
}
