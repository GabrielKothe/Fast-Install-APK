package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ---------------------------------------------------------------------------
// AndroidManifest.xml binario (AXML)
// ---------------------------------------------------------------------------

type xmlAttr struct {
	Name  string
	ResID uint32
	Value string
}

type xmlElem struct {
	Name  string
	Depth int
	Attrs []xmlAttr
}

// IDs de recurso dos atributos android:* (usados quando o nome foi ofuscado)
const (
	attrLabel       = 0x01010001
	attrName        = 0x01010003
	attrDebuggable  = 0x0101000f
	attrMinSdk      = 0x0101020c
	attrVersionCode = 0x0101021b
	attrVersionName = 0x0101021c
	attrTargetSdk   = 0x01010270
	attrCompileSdk  = 0x01010572
)

func (e xmlElem) attr(name string, resID uint32) string {
	for _, a := range e.Attrs {
		if resID != 0 && a.ResID == resID {
			return a.Value
		}
	}
	for _, a := range e.Attrs {
		if a.Name == name {
			return a.Value
		}
	}
	return ""
}

func parseStringPool(c []byte) []string {
	if len(c) < 28 {
		return nil
	}
	hs := int(binary.LittleEndian.Uint16(c[2:]))
	count := int(binary.LittleEndian.Uint32(c[8:]))
	utf8 := binary.LittleEndian.Uint32(c[16:])&0x100 != 0
	sStart := int(binary.LittleEndian.Uint32(c[20:]))
	out := make([]string, count)
	for i := 0; i < count; i++ {
		p := hs + i*4
		if p+4 > len(c) {
			break
		}
		o := sStart + int(binary.LittleEndian.Uint32(c[p:]))
		if o >= len(c) {
			continue
		}
		if utf8 {
			if c[o]&0x80 != 0 {
				o += 2
			} else {
				o++
			}
			if o+1 >= len(c) {
				continue
			}
			n := int(c[o])
			if n&0x80 != 0 {
				n = (n&0x7f)<<8 | int(c[o+1])
				o += 2
			} else {
				o++
			}
			if o+n <= len(c) {
				out[i] = string(c[o : o+n])
			}
		} else {
			if o+2 > len(c) {
				continue
			}
			n := int(binary.LittleEndian.Uint16(c[o:]))
			if n&0x8000 != 0 {
				n = (n&0x7fff)<<16 | int(binary.LittleEndian.Uint16(c[o+2:]))
				o += 4
			} else {
				o += 2
			}
			if o+n*2 > len(c) {
				continue
			}
			u := make([]uint16, n)
			for k := 0; k < n; k++ {
				u[k] = binary.LittleEndian.Uint16(c[o+k*2:])
			}
			out[i] = string(utf16.Decode(u))
		}
	}
	return out
}

func parseAXML(b []byte) ([]xmlElem, error) {
	if len(b) < 8 || binary.LittleEndian.Uint16(b) != 0x0003 {
		return nil, errors.New("AndroidManifest.xml nao esta no formato binario")
	}
	u16 := func(o int) int { return int(binary.LittleEndian.Uint16(b[o:])) }
	u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }

	var strs []string
	var resIDs []uint32
	str := func(i uint32) string {
		if int(i) < len(strs) {
			return strs[i]
		}
		return ""
	}
	var elems []xmlElem
	depth := 0
	pos := u16(2)
	for pos+8 <= len(b) {
		typ, hs, size := u16(pos), u16(pos+2), int(u32(pos+4))
		if size < 8 || pos+size > len(b) {
			break
		}
		switch typ {
		case 0x0001:
			strs = parseStringPool(b[pos : pos+size])
		case 0x0180:
			for o := pos + hs; o+4 <= pos+size; o += 4 {
				resIDs = append(resIDs, u32(o))
			}
		case 0x0102:
			ext := pos + hs
			if ext+20 > pos+size {
				break
			}
			el := xmlElem{Name: str(u32(ext + 4)), Depth: depth}
			aStart, aSize, aCount := u16(ext+8), u16(ext+10), u16(ext+12)
			for a := 0; a < aCount; a++ {
				ao := ext + aStart + a*aSize
				if ao+20 > pos+size {
					break
				}
				ni := u32(ao + 4)
				attr := xmlAttr{Name: str(ni)}
				if int(ni) < len(resIDs) {
					attr.ResID = resIDs[ni]
				}
				raw, dt, data := u32(ao+8), b[ao+15], u32(ao+16)
				switch {
				case raw != 0xFFFFFFFF:
					attr.Value = str(raw)
				case dt == 0x03:
					attr.Value = str(data)
				case dt == 0x10:
					attr.Value = strconv.Itoa(int(int32(data)))
				case dt == 0x11:
					attr.Value = fmt.Sprintf("0x%x", data)
				case dt == 0x12:
					attr.Value = strconv.FormatBool(data != 0)
				case dt == 0x01:
					attr.Value = fmt.Sprintf("@0x%08x", data)
				default:
					attr.Value = fmt.Sprintf("%d", data)
				}
				el.Attrs = append(el.Attrs, attr)
			}
			elems = append(elems, el)
			depth++
		case 0x0103:
			depth--
		}
		pos += size
	}
	if len(elems) == 0 || elems[0].Name != "manifest" {
		return nil, errors.New("manifesto invalido")
	}
	return elems, nil
}

// ---------------------------------------------------------------------------
// Informacoes do APK
// ---------------------------------------------------------------------------

type CertInfo struct {
	Scheme  string
	Subject string
	SHA1    string
	SHA256  string
	Expires string
}

type ApkInfo struct {
	Path           string
	Size           int64
	FileSHA256     string
	Package        string
	VersionName    string
	VersionCode    string
	MinSdk         string
	TargetSdk      string
	CompileSdk     string
	Label          string
	LaunchActivity string
	Debuggable     bool
	Permissions    []string
	ABIs           []string
	Cert           *CertInfo
}

func readApk(path string, full bool) (*ApkInfo, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel abrir o APK: %v", err)
	}
	defer zr.Close()

	info := &ApkInfo{Path: path}
	var manifest []byte
	abis := map[string]bool{}
	var v1Cert *zip.File
	for _, f := range zr.File {
		switch {
		case f.Name == "AndroidManifest.xml":
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			manifest, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(f.Name, "lib/"):
			if p := strings.Split(f.Name, "/"); len(p) >= 3 && p[1] != "" {
				abis[p[1]] = true
			}
		case strings.HasPrefix(f.Name, "META-INF/") && !strings.Contains(f.Name[9:], "/"):
			up := strings.ToUpper(f.Name)
			if strings.HasSuffix(up, ".RSA") || strings.HasSuffix(up, ".DSA") || strings.HasSuffix(up, ".EC") {
				v1Cert = f
			}
		}
	}
	if manifest == nil {
		return nil, errors.New("AndroidManifest.xml nao encontrado no APK")
	}
	elems, err := parseAXML(manifest)
	if err != nil {
		return nil, err
	}

	m := elems[0]
	info.Package = m.attr("package", 0)
	info.VersionCode = m.attr("versionCode", attrVersionCode)
	info.VersionName = m.attr("versionName", attrVersionName)
	info.CompileSdk = m.attr("compileSdkVersion", attrCompileSdk)
	if info.Package == "" {
		return nil, errors.New("package name nao encontrado no manifesto")
	}

	// activity com MAIN + LAUNCHER
	curAct, curDepth := "", -1
	hasMain, hasLauncher := false, false
	closeAct := func() {
		if curAct != "" && hasMain && hasLauncher && info.LaunchActivity == "" {
			info.LaunchActivity = curAct
		}
		curAct, curDepth, hasMain, hasLauncher = "", -1, false, false
	}
	for _, e := range elems[1:] {
		if curAct != "" && e.Depth <= curDepth {
			closeAct()
		}
		switch e.Name {
		case "uses-sdk":
			info.MinSdk = e.attr("minSdkVersion", attrMinSdk)
			info.TargetSdk = e.attr("targetSdkVersion", attrTargetSdk)
		case "uses-permission", "uses-permission-sdk-23":
			if n := e.attr("name", attrName); n != "" {
				info.Permissions = append(info.Permissions, n)
			}
		case "application":
			if l := e.attr("label", attrLabel); l != "" && !strings.HasPrefix(l, "@") {
				info.Label = l
			}
			info.Debuggable = e.attr("debuggable", attrDebuggable) == "true"
		case "activity", "activity-alias":
			curAct, curDepth = e.attr("name", attrName), e.Depth
			if strings.HasPrefix(curAct, ".") {
				curAct = info.Package + curAct
			}
		case "action":
			if curAct != "" && e.attr("name", attrName) == "android.intent.action.MAIN" {
				hasMain = true
			}
		case "category":
			if curAct != "" && e.attr("name", attrName) == "android.intent.category.LAUNCHER" {
				hasLauncher = true
			}
		}
	}
	closeAct()
	if info.MinSdk == "" {
		info.MinSdk = "1"
	}
	if info.TargetSdk == "" {
		info.TargetSdk = info.MinSdk
	}
	for a := range abis {
		info.ABIs = append(info.ABIs, a)
	}
	sort.Strings(info.ABIs)

	if !full {
		return info, nil
	}
	if st, err := os.Stat(path); err == nil {
		info.Size = st.Size()
	}
	if f, err := os.Open(path); err == nil {
		h := sha256.New()
		io.Copy(h, f)
		f.Close()
		info.FileSHA256 = hex.EncodeToString(h.Sum(nil))
	}
	info.Cert = readCert(path, v1Cert)
	return info, nil
}

// ---------------------------------------------------------------------------
// Certificado de assinatura (APK Signature Scheme v3/v2, ou v1/JAR)
// ---------------------------------------------------------------------------

func certFingerprint(der []byte, scheme string) *CertInfo {
	s1 := sha1.Sum(der)
	s256 := sha256.Sum256(der)
	ci := &CertInfo{Scheme: scheme, SHA1: colonHex(s1[:]), SHA256: colonHex(s256[:])}
	if c, err := x509.ParseCertificate(der); err == nil {
		ci.Subject = c.Subject.String()
		ci.Expires = c.NotAfter.Format("02/01/2006")
	}
	return ci
}

func colonHex(b []byte) string {
	s := strings.ToUpper(hex.EncodeToString(b))
	var parts []string
	for i := 0; i < len(s); i += 2 {
		parts = append(parts, s[i:i+2])
	}
	return strings.Join(parts, ":")
}

func readCertFile(path string) *CertInfo {
	zr, err := zip.OpenReader(path)
	var v1 *zip.File
	if err == nil {
		defer zr.Close()
		for _, f := range zr.File {
			up := strings.ToUpper(f.Name)
			if strings.HasPrefix(up, "META-INF/") && (strings.HasSuffix(up, ".RSA") || strings.HasSuffix(up, ".DSA") || strings.HasSuffix(up, ".EC")) {
				v1 = f
			}
		}
	}
	return readCert(path, v1)
}

func readCert(path string, v1 *zip.File) *CertInfo {
	if der, scheme := signingBlockCert(path); der != nil {
		return certFingerprint(der, scheme)
	}
	if v1 != nil {
		rc, err := v1.Open()
		if err == nil {
			data, _ := io.ReadAll(rc)
			rc.Close()
			if der := pkcs7FirstCert(data); der != nil {
				return certFingerprint(der, "v1 (JAR)")
			}
		}
	}
	return nil
}

func lenPrefixed(b []byte) (inner, rest []byte, ok bool) {
	if len(b) < 4 {
		return nil, nil, false
	}
	n := int(binary.LittleEndian.Uint32(b))
	if n < 0 || 4+n > len(b) {
		return nil, nil, false
	}
	return b[4 : 4+n], b[4+n:], true
}

func signingBlockCert(path string) ([]byte, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, ""
	}
	size := st.Size()
	tailLen := int64(65557)
	if size < tailLen {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := f.ReadAt(tail, size-tailLen); err != nil {
		return nil, ""
	}
	eocd := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:]) == 0x06054b50 {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, ""
	}
	cdOffset := int64(binary.LittleEndian.Uint32(tail[eocd+16:]))
	if cdOffset < 32 {
		return nil, ""
	}
	footer := make([]byte, 24)
	if _, err := f.ReadAt(footer, cdOffset-24); err != nil || string(footer[8:]) != "APK Sig Block 42" {
		return nil, ""
	}
	blockSize := int64(binary.LittleEndian.Uint64(footer))
	start := cdOffset - blockSize - 8
	if blockSize < 24 || start < 0 || blockSize > 64<<20 {
		return nil, ""
	}
	block := make([]byte, blockSize+8)
	if _, err := f.ReadAt(block, start); err != nil {
		return nil, ""
	}
	pairs := block[8 : len(block)-24]
	found := map[uint32][]byte{}
	for len(pairs) >= 12 {
		plen := int(binary.LittleEndian.Uint64(pairs))
		if plen < 4 || 8+plen > len(pairs) {
			break
		}
		id := binary.LittleEndian.Uint32(pairs[8:])
		found[id] = pairs[12 : 8+plen]
		pairs = pairs[8+plen:]
	}
	for _, s := range []struct {
		id   uint32
		name string
	}{{0xf05368c0, "v3"}, {0x7109871a, "v2"}} {
		v, ok := found[s.id]
		if !ok {
			continue
		}
		signers, _, ok := lenPrefixed(v)
		if !ok {
			continue
		}
		signer, _, ok := lenPrefixed(signers)
		if !ok {
			continue
		}
		signed, _, ok := lenPrefixed(signer)
		if !ok {
			continue
		}
		_, rest, ok := lenPrefixed(signed) // digests
		if !ok {
			continue
		}
		certs, _, ok := lenPrefixed(rest)
		if !ok {
			continue
		}
		cert, _, ok := lenPrefixed(certs)
		if ok && len(cert) > 0 {
			return cert, s.name
		}
	}
	return nil, ""
}

// Extrai o primeiro certificado de um PKCS#7 SignedData (DER)
func pkcs7FirstCert(data []byte) []byte {
	var ci asn1.RawValue
	if _, err := asn1.Unmarshal(data, &ci); err != nil {
		return nil
	}
	rest := ci.Bytes
	var oid, explicit asn1.RawValue
	if rest, _ = asn1.Unmarshal(rest, &oid); rest == nil {
		return nil
	}
	if _, err := asn1.Unmarshal(rest, &explicit); err != nil {
		return nil
	}
	var sd asn1.RawValue
	if _, err := asn1.Unmarshal(explicit.Bytes, &sd); err != nil {
		return nil
	}
	children := sd.Bytes
	for len(children) > 0 {
		var child asn1.RawValue
		r, err := asn1.Unmarshal(children, &child)
		if err != nil {
			return nil
		}
		children = r
		if child.Class == asn1.ClassContextSpecific && child.Tag == 0 {
			var cert asn1.RawValue
			if _, err := asn1.Unmarshal(child.Bytes, &cert); err == nil {
				return bytes.Clone(cert.FullBytes)
			}
		}
	}
	return nil
}
