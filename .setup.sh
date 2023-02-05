readonly base_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")"; pwd)"

readonly plat_os="$(uname -s | tr "A-Z" "a-z")"

case "$(uname -m)" in
    aarch64|arm64)
        readonly plat_arch=arm64
	;;
    x86_64|amd64)
        readonly plat_arch=amd64
	;;
    *)
        echo $'\e[31m'"unsupported platform: $(uname -m)"$'\e[0m' >&2
	exit 1
esac

readonly plat_dist="${base_dir}/dist/${plat_os}/${plat_arch}"
