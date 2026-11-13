#!/bin/bash
# Remove as Acoes Rapidas e os arquivos instalados
DEST="$HOME/Library/Application Support/FastInstallAPK"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
[ -x "$LSREGISTER" ] && "$LSREGISTER" -u "$DEST/FastInstallAPK.app" >/dev/null 2>&1
rm -rf "$HOME/Library/Services/Instalar via adb.workflow" "$HOME/Library/Services/Desinstalar via adb.workflow" "$DEST"
/System/Library/CoreServices/pbs -update >/dev/null 2>&1 || true
echo "Fast Install APK removido."
