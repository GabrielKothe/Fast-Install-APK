#!/bin/bash
# Instala as Acoes Rapidas "Instalar via adb" / "Desinstalar via adb" no Finder.
# Uso: ./install.sh            -> aparece so em arquivos .apk
#      ./install.sh --all-files -> aparece em qualquer arquivo (use se o filtro .apk nao funcionar)
set -e

SRC="$(cd "$(dirname "$0")" && pwd)"
DEST="$HOME/Library/Application Support/FastInstallAPK"
SERVICES="$HOME/Library/Services"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
UTI="com.android.package-archive"
[ "$1" = "--all-files" ] && UTI="public.data"

mkdir -p "$DEST" "$SERVICES"
cp "$SRC/fast-apk.sh" "$DEST/fast-apk.sh"
chmod +x "$DEST/fast-apk.sh"

# App "fantasma" que so declara o tipo .apk (UTI) para o macOS reconhecer o arquivo
APP="$DEST/FastInstallAPK.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS"
printf '#!/bin/bash\nexit 0\n' > "$APP/Contents/MacOS/FastInstallAPK"
chmod +x "$APP/Contents/MacOS/FastInstallAPK"
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key><string>com.fastinstallapk.types</string>
	<key>CFBundleName</key><string>FastInstallAPK</string>
	<key>CFBundleExecutable</key><string>FastInstallAPK</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>1.0</string>
	<key>LSBackgroundOnly</key><true/>
	<key>UTImportedTypeDeclarations</key>
	<array>
		<dict>
			<key>UTTypeIdentifier</key><string>com.android.package-archive</string>
			<key>UTTypeDescription</key><string>Android Package</string>
			<key>UTTypeConformsTo</key><array><string>public.data</string><string>public.zip-archive</string></array>
			<key>UTTypeTagSpecification</key>
			<dict>
				<key>public.filename-extension</key><array><string>apk</string></array>
				<key>public.mime-type</key><array><string>application/vnd.android.package-archive</string></array>
			</dict>
		</dict>
	</array>
</dict>
</plist>
PLIST
[ -x "$LSREGISTER" ] && "$LSREGISTER" -f "$APP" || true

xml_escape() { printf '%s' "$1" | sed 's/&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g'; }

make_workflow() {  # $1 = nome do menu, $2 = acao
  local wf="$SERVICES/$1.workflow"
  rm -rf "$wf"
  mkdir -p "$wf/Contents"
  local script
  script=$(xml_escape "for f in \"\$@\"; do
  case \"\$f\" in *.apk|*.APK) \"$DEST/fast-apk.sh\" launch $2 \"\$f\" ;; esac
done")

  cat > "$wf/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>NSServices</key>
	<array>
		<dict>
			<key>NSBackgroundColorName</key><string>background</string>
			<key>NSIconName</key><string>NSActionTemplate</string>
			<key>NSMenuItem</key><dict><key>default</key><string>$1</string></dict>
			<key>NSMessage</key><string>runWorkflowAsService</string>
			<key>NSRequiredContext</key><dict><key>NSApplicationIdentifier</key><string>com.apple.finder</string></dict>
			<key>NSSendFileTypes</key><array><string>$UTI</string></array>
		</dict>
	</array>
</dict>
</plist>
EOF

  cat > "$wf/Contents/document.wflow" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>AMApplicationBuild</key><string>523</string>
	<key>AMApplicationVersion</key><string>2.10</string>
	<key>AMDocumentVersion</key><string>2</string>
	<key>actions</key>
	<array>
		<dict>
			<key>action</key>
			<dict>
				<key>AMAccepts</key>
				<dict>
					<key>Container</key><string>List</string>
					<key>Optional</key><true/>
					<key>Types</key><array><string>com.apple.cocoa.string</string></array>
				</dict>
				<key>AMActionVersion</key><string>2.0.3</string>
				<key>AMApplication</key><array><string>Automator</string></array>
				<key>AMParameterProperties</key>
				<dict>
					<key>COMMAND_STRING</key><dict/>
					<key>CheckedForUserDefaultShell</key><dict/>
					<key>inputMethod</key><dict/>
					<key>shell</key><dict/>
					<key>source</key><dict/>
				</dict>
				<key>AMProvides</key>
				<dict>
					<key>Container</key><string>List</string>
					<key>Types</key><array><string>com.apple.cocoa.string</string></array>
				</dict>
				<key>ActionBundlePath</key><string>/System/Library/Automator/Run Shell Script.action</string>
				<key>ActionName</key><string>Run Shell Script</string>
				<key>ActionParameters</key>
				<dict>
					<key>COMMAND_STRING</key><string>$script</string>
					<key>CheckedForUserDefaultShell</key><true/>
					<key>inputMethod</key><integer>1</integer>
					<key>shell</key><string>/bin/bash</string>
					<key>source</key><string></string>
				</dict>
				<key>BundleIdentifier</key><string>com.apple.RunShellScript</string>
				<key>CFBundleVersion</key><string>2.0.3</string>
				<key>CanShowSelectedItemsWhenRun</key><false/>
				<key>CanShowWhenRun</key><true/>
				<key>Category</key><array><string>AMCategoryUtilities</string></array>
				<key>Class Name</key><string>RunShellScriptAction</string>
				<key>InputUUID</key><string>$(uuidgen)</string>
				<key>Keywords</key><array><string>Shell</string><string>Script</string></array>
				<key>OutputUUID</key><string>$(uuidgen)</string>
				<key>UUID</key><string>$(uuidgen)</string>
				<key>UnlocalizedApplications</key><array><string>Automator</string></array>
				<key>arguments</key><dict/>
				<key>isViewVisible</key><integer>1</integer>
				<key>location</key><string>309.000000:253.000000</string>
				<key>nibPath</key><string>/System/Library/Automator/Run Shell Script.action/Contents/Resources/Base.lproj/main.nib</string>
			</dict>
			<key>isViewVisible</key><integer>1</integer>
		</dict>
	</array>
	<key>connectors</key><dict/>
	<key>workflowMetaData</key>
	<dict>
		<key>applicationBundleIDsByPath</key><dict/>
		<key>applicationPaths</key><array/>
		<key>inputTypeIdentifier</key><string>com.apple.Automator.fileSystemObject</string>
		<key>outputTypeIdentifier</key><string>com.apple.Automator.nothing</string>
		<key>presentationMode</key><integer>15</integer>
		<key>processesInput</key><integer>0</integer>
		<key>serviceInputTypeIdentifier</key><string>com.apple.Automator.fileSystemObject</string>
		<key>serviceOutputTypeIdentifier</key><string>com.apple.Automator.nothing</string>
		<key>serviceProcessesInput</key><integer>0</integer>
		<key>systemImageName</key><string>NSActionTemplate</string>
		<key>useAutomaticInputType</key><integer>0</integer>
		<key>workflowTypeIdentifier</key><string>com.apple.Automator.servicesMenu</string>
	</dict>
</dict>
</plist>
EOF
}

make_workflow "Instalar via adb" install
make_workflow "Desinstalar via adb" uninstall

/System/Library/CoreServices/pbs -flush >/dev/null 2>&1 || true
/System/Library/CoreServices/pbs -update >/dev/null 2>&1 || true

echo "Fast Install APK instalado!"
echo "Clique com o botao direito em um .apk no Finder > Acoes Rapidas > Instalar/Desinstalar via adb."
echo "Se nao aparecer: Ajustes do Sistema > Privacidade e Seguranca > Extensoes > Finder, e ative as duas acoes."
