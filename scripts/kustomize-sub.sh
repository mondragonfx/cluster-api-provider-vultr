root=$(dirname "$0")
bin_dir="$root/../bin"

kustomize=$(ls "$bin_dir"/kustomize-* 2>/dev/null | head -1)
envsubst=$(ls "$bin_dir"/envsubst-* 2>/dev/null | head -1)

"$kustomize" build "$1" | "$envsubst"