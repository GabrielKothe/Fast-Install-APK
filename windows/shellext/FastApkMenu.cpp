// Fast Install APK - menu de contexto moderno do Windows 11 (IExplorerCommand)
// Carregada pelo Explorer via pacote MSIX "sparse" (ver AppxManifest.xml).
//
// Mostra "Fast Install APK" com um submenu. Os itens vem de menu.txt (gerado por `fastapk menu`),
// uma linha por item: id|titulo|icone|flags   (flags: s = separador antes, 1 = so o primeiro arquivo)
// Cada item executa: fastapk.exe <id> "<arquivo.apk>"
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <shobjidl.h>
#include <new>
#include <string>
#include <vector>

static HMODULE g_module = nullptr;
static LONG g_refs = 0;

// {A3C1F0E2-5B7D-4E8A-9C61-2D4F8B9E1A01}
static const CLSID CLSID_Root = {0xa3c1f0e2, 0x5b7d, 0x4e8a, {0x9c, 0x61, 0x2d, 0x4f, 0x8b, 0x9e, 0x1a, 0x01}};

struct MenuItem {
    std::wstring id, title, icon;
    bool separator = false;
    bool single = false;
};

static std::wstring ModuleDir() {
    wchar_t path[MAX_PATH * 2];
    DWORD n = GetModuleFileNameW(g_module, path, ARRAYSIZE(path));
    std::wstring s(path, n);
    size_t p = s.find_last_of(L"\\/");
    return p == std::wstring::npos ? s : s.substr(0, p);
}

static std::wstring Utf8ToWide(const std::string& s) {
    if (s.empty()) return L"";
    int n = MultiByteToWideChar(CP_UTF8, 0, s.data(), (int)s.size(), nullptr, 0);
    std::wstring w(n, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, s.data(), (int)s.size(), &w[0], n);
    return w;
}

static std::vector<MenuItem> LoadMenu() {
    std::vector<MenuItem> items;
    std::wstring file = ModuleDir() + L"\\menu.txt";
    HANDLE h = CreateFileW(file.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, 0, nullptr);
    if (h == INVALID_HANDLE_VALUE) return items;
    std::string data;
    char buf[4096];
    DWORD got = 0;
    while (ReadFile(h, buf, sizeof(buf), &got, nullptr) && got > 0) data.append(buf, got);
    CloseHandle(h);

    std::wstring text = Utf8ToWide(data);
    size_t pos = 0;
    while (pos < text.size()) {
        size_t end = text.find(L'\n', pos);
        if (end == std::wstring::npos) end = text.size();
        std::wstring line = text.substr(pos, end - pos);
        pos = end + 1;
        while (!line.empty() && (line.back() == L'\r' || line.back() == L' ')) line.pop_back();
        if (line.empty()) continue;
        std::vector<std::wstring> f;
        size_t s = 0, bar;
        while ((bar = line.find(L'|', s)) != std::wstring::npos) { f.push_back(line.substr(s, bar - s)); s = bar + 1; }
        f.push_back(line.substr(s));
        if (f.size() < 2) continue;
        MenuItem it;
        it.id = f[0];
        it.title = f[1];
        if (f.size() > 2) it.icon = f[2];
        if (f.size() > 3) {
            it.separator = f[3].find(L's') != std::wstring::npos;
            it.single = f[3].find(L'1') != std::wstring::npos;
        }
        items.push_back(it);
    }
    return items;
}

static HRESULT DupString(const std::wstring& s, LPWSTR* out) {
    size_t bytes = (s.size() + 1) * sizeof(wchar_t);
    *out = static_cast<LPWSTR>(CoTaskMemAlloc(bytes));
    if (!*out) return E_OUTOFMEMORY;
    memcpy(*out, s.c_str(), bytes);
    return S_OK;
}

// ---------------------------------------------------------------------------
// Base com contagem de referencia
// ---------------------------------------------------------------------------
class CommandBase : public IExplorerCommand {
public:
    CommandBase() : m_refs(1) { InterlockedIncrement(&g_refs); }
    virtual ~CommandBase() { InterlockedDecrement(&g_refs); }

    IFACEMETHODIMP QueryInterface(REFIID riid, void** ppv) override {
        if (!ppv) return E_POINTER;
        if (riid == IID_IUnknown || riid == IID_IExplorerCommand) {
            *ppv = static_cast<IExplorerCommand*>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    IFACEMETHODIMP_(ULONG) AddRef() override { return InterlockedIncrement(&m_refs); }
    IFACEMETHODIMP_(ULONG) Release() override {
        LONG r = InterlockedDecrement(&m_refs);
        if (r == 0) delete this;
        return r;
    }
    IFACEMETHODIMP GetToolTip(IShellItemArray*, LPWSTR* tip) override { *tip = nullptr; return E_NOTIMPL; }
    IFACEMETHODIMP GetCanonicalName(GUID* guid) override { *guid = GUID_NULL; return E_NOTIMPL; }
    IFACEMETHODIMP GetState(IShellItemArray*, BOOL, EXPCMDSTATE* state) override { *state = ECS_ENABLED; return S_OK; }

private:
    LONG m_refs;
};

// ---------------------------------------------------------------------------
// Item do submenu (ou separador)
// ---------------------------------------------------------------------------
class SubCommand : public CommandBase {
public:
    explicit SubCommand(const MenuItem& item, bool isSeparator) : m_item(item), m_sep(isSeparator) {}

    IFACEMETHODIMP GetTitle(IShellItemArray*, LPWSTR* name) override {
        if (m_sep) { *name = nullptr; return S_FALSE; }
        return DupString(m_item.title, name);
    }
    IFACEMETHODIMP GetIcon(IShellItemArray*, LPWSTR* icon) override {
        *icon = nullptr;
        if (m_sep || m_item.icon.empty()) return E_NOTIMPL;
        return DupString(ModuleDir() + L"\\" + m_item.icon, icon);
    }
    IFACEMETHODIMP GetFlags(EXPCMDFLAGS* flags) override { *flags = m_sep ? ECF_ISSEPARATOR : ECF_DEFAULT; return S_OK; }
    IFACEMETHODIMP EnumSubCommands(IEnumExplorerCommand** e) override { *e = nullptr; return E_NOTIMPL; }

    IFACEMETHODIMP Invoke(IShellItemArray* items, IBindCtx*) override {
        if (m_sep || !items) return S_OK;
        DWORD count = 0;
        items->GetCount(&count);
        if (m_item.single && count > 1) count = 1;
        std::wstring exe = ModuleDir() + L"\\fastapk.exe";
        for (DWORD i = 0; i < count; i++) {
            IShellItem* item = nullptr;
            if (FAILED(items->GetItemAt(i, &item))) continue;
            LPWSTR path = nullptr;
            if (SUCCEEDED(item->GetDisplayName(SIGDN_FILESYSPATH, &path))) {
                std::wstring cmd = L"\"" + exe + L"\" " + m_item.id + L" \"" + path + L"\"";
                STARTUPINFOW si = {sizeof(si)};
                PROCESS_INFORMATION pi = {};
                if (CreateProcessW(exe.c_str(), &cmd[0], nullptr, nullptr, FALSE, CREATE_NEW_CONSOLE, nullptr, nullptr, &si, &pi)) {
                    CloseHandle(pi.hThread);
                    CloseHandle(pi.hProcess);
                }
                CoTaskMemFree(path);
            }
            item->Release();
        }
        return S_OK;
    }

private:
    MenuItem m_item;
    bool m_sep;
};

// ---------------------------------------------------------------------------
// Enumerador dos itens do submenu
// ---------------------------------------------------------------------------
class EnumCommands : public IEnumExplorerCommand {
public:
    explicit EnumCommands(std::vector<IExplorerCommand*> cmds) : m_refs(1), m_cmds(std::move(cmds)), m_pos(0) {
        InterlockedIncrement(&g_refs);
    }
    virtual ~EnumCommands() {
        for (auto* c : m_cmds) c->Release();
        InterlockedDecrement(&g_refs);
    }
    IFACEMETHODIMP QueryInterface(REFIID riid, void** ppv) override {
        if (riid == IID_IUnknown || riid == IID_IEnumExplorerCommand) {
            *ppv = static_cast<IEnumExplorerCommand*>(this);
            AddRef();
            return S_OK;
        }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    IFACEMETHODIMP_(ULONG) AddRef() override { return InterlockedIncrement(&m_refs); }
    IFACEMETHODIMP_(ULONG) Release() override {
        LONG r = InterlockedDecrement(&m_refs);
        if (r == 0) delete this;
        return r;
    }
    IFACEMETHODIMP Next(ULONG celt, IExplorerCommand** out, ULONG* fetched) override {
        ULONG n = 0;
        while (n < celt && m_pos < m_cmds.size()) {
            out[n] = m_cmds[m_pos++];
            out[n]->AddRef();
            n++;
        }
        if (fetched) *fetched = n;
        return n == celt ? S_OK : S_FALSE;
    }
    IFACEMETHODIMP Skip(ULONG celt) override {
        m_pos += celt;
        if (m_pos > m_cmds.size()) { m_pos = m_cmds.size(); return S_FALSE; }
        return S_OK;
    }
    IFACEMETHODIMP Reset() override { m_pos = 0; return S_OK; }
    IFACEMETHODIMP Clone(IEnumExplorerCommand** out) override { *out = nullptr; return E_NOTIMPL; }

private:
    LONG m_refs;
    std::vector<IExplorerCommand*> m_cmds;
    size_t m_pos;
};

// ---------------------------------------------------------------------------
// Item raiz "Fast Install APK"
// ---------------------------------------------------------------------------
class RootCommand : public CommandBase {
public:
    IFACEMETHODIMP GetTitle(IShellItemArray*, LPWSTR* name) override { return DupString(L"Fast Install APK", name); }
    IFACEMETHODIMP GetIcon(IShellItemArray*, LPWSTR* icon) override { return DupString(ModuleDir() + L"\\logo.ico", icon); }
    IFACEMETHODIMP GetCanonicalName(GUID* guid) override { *guid = CLSID_Root; return S_OK; }
    IFACEMETHODIMP GetFlags(EXPCMDFLAGS* flags) override { *flags = ECF_HASSUBCOMMANDS; return S_OK; }
    IFACEMETHODIMP Invoke(IShellItemArray*, IBindCtx*) override { return S_OK; }

    IFACEMETHODIMP EnumSubCommands(IEnumExplorerCommand** e) override {
        std::vector<IExplorerCommand*> cmds;
        bool first = true;
        for (const auto& it : LoadMenu()) {
            if (it.separator && !first) {
                if (auto* s = new (std::nothrow) SubCommand(it, true)) cmds.push_back(s);
            }
            if (auto* c = new (std::nothrow) SubCommand(it, false)) cmds.push_back(c);
            first = false;
        }
        *e = new (std::nothrow) EnumCommands(std::move(cmds));
        return *e ? S_OK : E_OUTOFMEMORY;
    }
};

// ---------------------------------------------------------------------------
class Factory : public IClassFactory {
public:
    Factory() : m_refs(1) { InterlockedIncrement(&g_refs); }
    virtual ~Factory() { InterlockedDecrement(&g_refs); }
    IFACEMETHODIMP QueryInterface(REFIID riid, void** ppv) override {
        if (riid == IID_IUnknown || riid == IID_IClassFactory) { *ppv = this; AddRef(); return S_OK; }
        *ppv = nullptr;
        return E_NOINTERFACE;
    }
    IFACEMETHODIMP_(ULONG) AddRef() override { return InterlockedIncrement(&m_refs); }
    IFACEMETHODIMP_(ULONG) Release() override {
        LONG r = InterlockedDecrement(&m_refs);
        if (r == 0) delete this;
        return r;
    }
    IFACEMETHODIMP CreateInstance(IUnknown* outer, REFIID riid, void** ppv) override {
        if (outer) return CLASS_E_NOAGGREGATION;
        RootCommand* cmd = new (std::nothrow) RootCommand();
        if (!cmd) return E_OUTOFMEMORY;
        HRESULT hr = cmd->QueryInterface(riid, ppv);
        cmd->Release();
        return hr;
    }
    IFACEMETHODIMP LockServer(BOOL lock) override {
        if (lock) InterlockedIncrement(&g_refs); else InterlockedDecrement(&g_refs);
        return S_OK;
    }

private:
    LONG m_refs;
};

extern "C" {

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        g_module = inst;
        DisableThreadLibraryCalls(inst);
    }
    return TRUE;
}

STDAPI DllGetClassObject(REFCLSID clsid, REFIID riid, void** ppv) {
    if (!ppv) return E_POINTER;
    *ppv = nullptr;
    if (clsid != CLSID_Root) return CLASS_E_CLASSNOTAVAILABLE;
    Factory* f = new (std::nothrow) Factory();
    if (!f) return E_OUTOFMEMORY;
    HRESULT hr = f->QueryInterface(riid, ppv);
    f->Release();
    return hr;
}

STDAPI DllCanUnloadNow() { return g_refs == 0 ? S_OK : S_FALSE; }
}
