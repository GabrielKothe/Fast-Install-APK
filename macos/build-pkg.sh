#!/bin/bash
# Gera dist/FastInstallAPK-<versao>.pkg  (precisa rodar no macOS)
# Uso: bash macos/build-pkg.sh [versao]
set -e
VERSION="${1:-1.0.0}"
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/../dist"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PAYLOAD="$WORK/root/Library/Application Support/FastInstallAPK/installer"
mkdir -p "$PAYLOAD" "$WORK/scripts" "$OUT"
cp "${FASTAPK_BIN:-$HERE/fastapk}" "$HERE/install.sh" "$HERE/uninstall.sh" "$PAYLOAD/"
chmod 755 "$PAYLOAD"/*

# As Acoes Rapidas ficam na pasta do usuario, entao o postinstall roda o install.sh como o usuario logado
cat > "$WORK/scripts/postinstall" <<'EOF'
#!/bin/bash
CONSOLE_USER=$(stat -f%Su /dev/console)
[ -z "$CONSOLE_USER" ] || [ "$CONSOLE_USER" = "root" ] && exit 0
sudo -u "$CONSOLE_USER" -H bash "/Library/Application Support/FastInstallAPK/installer/install.sh"
exit 0
EOF
chmod 755 "$WORK/scripts/postinstall"

pkgbuild --root "$WORK/root" --scripts "$WORK/scripts" \
  --identifier com.fastinstallapk.pkg --version "$VERSION" --install-location / \
  "$OUT/FastInstallAPK-$VERSION.pkg"
echo "Gerado: $OUT/FastInstallAPK-$VERSION.pkg"
