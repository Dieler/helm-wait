#!/bin/sh -e

if [ -n "${HELM_LINTER_PLUGIN_NO_INSTALL_HOOK}" ]; then
    echo "Development mode: not downloading versioned release."
    exit 0
fi

# shellcheck disable=SC2002
version="$(cat plugin.yaml | grep "version" | cut -d '"' -f 2)"
echo "Downloading and installing helm-wait v${version} ..."

url=""
if [ "$(uname)" = "Darwin" ]; then
    url="https://github.com/Dieler/helm-wait/releases/download/v${version}/helm-wait-macos.tar.gz"
elif [ "$(uname)" = "Linux" ] ; then
    url="https://github.com/Dieler/helm-wait/releases/download/v${version}/helm-wait-linux.tar.gz"
else
    url="https://github.com/Dieler/helm-wait/releases/download/v${version}/helm-wait-windows.tar.gz"
fi

echo "$url"

mkdir -p "bin"
mkdir -p "releases/v${version}"

# Download with curl if possible.
# shellcheck disable=SC2230
if [ -x "$(which curl 2>/dev/null)" ]; then
    curl -sSL "${url}" -o "releases/v${version}.tar.gz"
else
    wget -q "${url}" -O "releases/v${version}.tar.gz"
fi
tar xzf "releases/v${version}.tar.gz" -C "releases/v${version}"
mv "releases/v${version}/bin/wait" "bin/wait" || \
    mv "releases/v${version}/bin/wait.exe" "bin/wait"
mv "releases/v${version}/plugin.yaml" .
mv "releases/v${version}/README.md" .
mv "releases/v${version}/LICENSE" .
