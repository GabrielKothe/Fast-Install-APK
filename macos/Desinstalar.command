#!/bin/bash
# Duplo clique para remover o Fast Install APK
if [ -f "$(dirname "$0")/uninstall.sh" ]; then
  bash "$(dirname "$0")/uninstall.sh"
else
  bash "/Library/Application Support/FastInstallAPK/installer/uninstall.sh"
fi
echo
read -r -p "Pressione Enter para fechar " _
