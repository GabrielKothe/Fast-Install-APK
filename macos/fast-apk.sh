#!/bin/bash
# Fast Install APK - macOS
# Uso: fast-apk.sh install|uninstall|package <apk>
#      fast-apk.sh launch install|uninstall <apk>   (abre uma janela do Terminal)

SELF="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
TITLE="Fast Install APK"

step() { printf '\033[36m==> %s\033[0m\n' "$*"; }
ok()   { printf '\033[32mOK  %s\033[0m\n' "$*"; }
err()  { printf '\033[31mERRO %s\033[0m\n' "$*"; }
warn() { printf '\033[33m%s\033[0m\n' "$*"; }

finish() {
  if [ "$1" -eq 0 ]; then
    printf '\nFechando em 3 segundos...\n'; sleep 3
  else
    printf '\n'; read -r -p 'Pressione Enter para fechar ' _
  fi
  exit "$1"
}

osa_escape() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }

confirm_box() {  # $1 = texto
  local r
  r=$(osascript -e "display dialog \"$(osa_escape "$1")\" with title \"$TITLE\" buttons {\"Cancelar\", \"Sim\"} default button \"Cancelar\" cancel button \"Cancelar\" with icon caution" 2>/dev/null)
  [[ "$r" == *"Sim"* ]]
}

find_adb() {
  if command -v adb >/dev/null 2>&1; then command -v adb; return; fi
  local c
  for c in "$ANDROID_HOME/platform-tools/adb" "$ANDROID_SDK_ROOT/platform-tools/adb" \
           "$HOME/Library/Android/sdk/platform-tools/adb" "/opt/homebrew/bin/adb" "/usr/local/bin/adb"; do
    [ -x "$c" ] && { echo "$c"; return; }
  done
  return 1
}

find_aapt() {
  local c
  for c in "$ANDROID_HOME" "$ANDROID_SDK_ROOT" "$HOME/Library/Android/sdk"; do
    [ -d "$c/build-tools" ] || continue
    local t; t=$(ls -d "$c"/build-tools/*/aapt 2>/dev/null | sort -V | tail -n1)
    [ -n "$t" ] && { echo "$t"; return; }
  done
  return 1
}

# Le o package name do AndroidManifest.xml binario usando JavaScript for Automation (nativo do macOS)
read_package() {
  local apk="$1" tmp pkg
  tmp=$(mktemp -d)
  if unzip -p "$apk" AndroidManifest.xml > "$tmp/m.bin" 2>/dev/null && [ -s "$tmp/m.bin" ]; then
    xxd -p "$tmp/m.bin" > "$tmp/m.hex"
    cat > "$tmp/p.js" <<'JS'
function run(argv) {
  ObjC.import('Foundation');
  var hex = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null).js.replace(/\s+/g, '');
  var n = hex.length / 2, b = new Uint8Array(n);
  for (var i = 0; i < n; i++) b[i] = parseInt(hex.substr(i * 2, 2), 16);
  function u16(o) { return b[o] | (b[o + 1] << 8); }
  function u32(o) { return (b[o] | (b[o + 1] << 8) | (b[o + 2] << 16) | (b[o + 3] << 24)) >>> 0; }
  var pool = null;
  function str(i) {
    if (!pool || i >= pool.count) return null;
    var o = pool.start + pool.sStart + u32(pool.start + pool.hsize + i * 4), s = '', k, len;
    if (pool.utf8) {
      o += (b[o] & 0x80) ? 2 : 1;
      len = b[o];
      if (len & 0x80) { len = ((len & 0x7f) << 8) | b[o + 1]; o += 2; } else { o += 1; }
      for (k = 0; k < len; k++) s += '%' + ('0' + b[o + k].toString(16)).slice(-2);
      try { return decodeURIComponent(s); } catch (e) { return null; }
    }
    len = u16(o);
    if (len & 0x8000) { len = ((len & 0x7fff) << 16) | u16(o + 2); o += 4; } else { o += 2; }
    for (k = 0; k < len; k++) s += String.fromCharCode(u16(o + k * 2));
    return s;
  }
  var pos = u16(2);
  while (pos < n) {
    var type = u16(pos), hsize = u16(pos + 2), size = u32(pos + 4);
    if (!size) break;
    if (type === 0x0001) {
      pool = { start: pos, hsize: hsize, count: u32(pos + 8), utf8: (u32(pos + 16) & 0x100) !== 0, sStart: u32(pos + 20) };
    } else if (type === 0x0102) {
      var ext = pos + hsize;
      if (str(u32(ext + 4)) !== 'manifest') break;
      var aStart = u16(ext + 8), aSize = u16(ext + 10), aCount = u16(ext + 12);
      for (var a = 0; a < aCount; a++) {
        var ao = ext + aStart + a * aSize;
        if (str(u32(ao + 4)) === 'package') {
          var raw = u32(ao + 8);
          if (raw !== 0xFFFFFFFF) return str(raw);
          if (b[ao + 15] === 3) return str(u32(ao + 16));
        }
      }
      break;
    }
    pos += size;
  }
  return '';
}
JS
    pkg=$(osascript -l JavaScript "$tmp/p.js" "$tmp/m.hex" 2>/dev/null)
    [ "$pkg" = "null" ] && pkg=""
  fi
  rm -rf "$tmp"
  if [ -z "$pkg" ]; then
    local aapt; aapt=$(find_aapt) && pkg=$("$aapt" dump badging "$apk" 2>/dev/null | sed -n "s/^package: name='\([^']*\)'.*/\1/p")
  fi
  [ -n "$pkg" ] && echo "$pkg"
}

select_devices() {
  local ready=() other=() serial state
  while read -r serial state _; do
    [ -z "$serial" ] && continue
    [ "$serial" = "List" ] && continue
    if [ "$state" = "device" ]; then ready+=("$serial"); else other+=("$serial ($state)"); fi
  done < <("$ADB" devices 2>/dev/null)

  if [ ${#ready[@]} -eq 0 ]; then
    err "Nenhum dispositivo pronto encontrado."
    if [ ${#other[@]} -gt 0 ]; then
      warn "Dispositivos com problema: ${other[*]}"
      warn 'Se aparecer "unauthorized", aceite a depuracao USB no celular.'
    fi
    finish 1
  fi
  if [ ${#ready[@]} -eq 1 ]; then DEVICES=("${ready[0]}"); return; fi

  printf '\n'; warn "Mais de um dispositivo conectado:"
  local i=1 d model
  for d in "${ready[@]}"; do
    model=$("$ADB" -s "$d" shell getprop ro.product.model 2>/dev/null | tr -d '\r')
    printf '  [%d] %s  %s\n' "$i" "$d" "$model"; i=$((i + 1))
  done
  printf '  [0] Todos\n'
  local sel
  while true; do
    read -r -p 'Escolha o dispositivo: ' sel
    if [ "$sel" = "0" ]; then DEVICES=("${ready[@]}"); return; fi
    if [[ "$sel" =~ ^[0-9]+$ ]] && [ "$sel" -ge 1 ] && [ "$sel" -le ${#ready[@]} ]; then
      DEVICES=("${ready[$((sel - 1))]}"); return
    fi
  done
}

is_installed() { "$ADB" -s "$1" shell pm list packages "$2" 2>/dev/null | tr -d '\r' | grep -qx "package:$2"; }

install_on() {
  local s="$1" out
  step "Instalando em $s ..."
  out=$("$ADB" -s "$s" install -r "$APK" 2>&1); echo "$out" | sed 's/^/    /'
  if echo "$out" | grep -qE 'INSTALL_FAILED_UPDATE_INCOMPATIBLE|INSTALL_FAILED_VERSION_DOWNGRADE'; then
    local why="a assinatura e diferente da versao instalada"
    echo "$out" | grep -q DOWNGRADE && why="a versao instalada e mais nova"
    if confirm_box "Nao foi possivel atualizar $PKG porque $why.

Desinstalar a versao atual (os dados do app serao apagados) e instalar de novo?"; then
      "$ADB" -s "$s" uninstall "$PKG" 2>&1 | sed 's/^/    /'
      out=$("$ADB" -s "$s" install -r "$APK" 2>&1); echo "$out" | sed 's/^/    /'
    fi
  fi
  if ! echo "$out" | grep -q '^Success'; then err "Falha ao instalar em $s."; return 1; fi
  ok "Instalado em $s."

  step "Abrindo $PKG ..."
  out=$("$ADB" -s "$s" shell monkey -p "$PKG" -c android.intent.category.LAUNCHER 1 2>&1)
  if echo "$out" | grep -qE 'No activities found|monkey aborted'; then
    warn "    App instalado, mas nao tem activity de launcher para abrir."
  else ok "App aberto."; fi
}

uninstall_from() {
  local s="$1" out
  if ! is_installed "$s" "$PKG"; then warn "    $PKG nao esta instalado em $s."; return 0; fi
  step "Desinstalando $PKG de $s ..."
  out=$("$ADB" -s "$s" uninstall "$PKG" 2>&1); echo "$out" | sed 's/^/    /'
  if ! echo "$out" | grep -q '^Success'; then err "Falha ao desinstalar de $s."; return 1; fi
  ok "Desinstalado de $s."
}

# ---------------------------------------------------------------------------
ACTION="$1"

if [ "$ACTION" = "launch" ]; then
  # Chamado pela Acao Rapida do Finder: abre uma janela do Terminal para mostrar o progresso
  cmdfile="$(mktemp -t fastapk).command"
  printf '#!/bin/bash\nrm -f "$0"\nclear\n%q %q %q\n' "$SELF" "$2" "$3" > "$cmdfile"
  chmod +x "$cmdfile"
  open -a Terminal "$cmdfile"
  exit 0
fi

APK="$2"
case "$ACTION" in install|uninstall|package) ;; *) echo "Uso: $0 install|uninstall|package <apk>"; exit 2 ;; esac
[ -f "$APK" ] || { err "Arquivo nao encontrado: $APK"; [ "$ACTION" = package ] && exit 1; finish 1; }
APK="$(cd "$(dirname "$APK")" && pwd)/$(basename "$APK")"

PKG=$(read_package "$APK")
if [ "$ACTION" = "package" ]; then [ -n "$PKG" ] && echo "$PKG" && exit 0; exit 1; fi

printf '\033]0;%s - %s\007' "$TITLE" "${PKG:-?}"
echo "APK:     $APK"
[ -z "$PKG" ] && { err "Nao foi possivel ler o package name do APK."; finish 1; }
echo "Package: $PKG"

ADB=$(find_adb) || { err "adb nao encontrado. Instale com: brew install --cask android-platform-tools"; finish 1; }

select_devices
rc=0
if [ "$ACTION" = "install" ]; then
  for d in "${DEVICES[@]}"; do install_on "$d" || rc=1; done
else
  target="${DEVICES[0]}"; [ ${#DEVICES[@]} -gt 1 ] && target="${#DEVICES[@]} dispositivos"
  if ! confirm_box "Desinstalar o app abaixo?

$PKG

Dispositivo: $target

Os dados do app serao apagados."; then
    warn "Cancelado pelo usuario."; finish 0
  fi
  for d in "${DEVICES[@]}"; do uninstall_from "$d" || rc=1; done
fi
finish $rc
