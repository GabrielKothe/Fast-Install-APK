// Fast Install APK - extensao do menu de contexto moderno do Windows 11 (IExplorerCommand)
// Carregada pelo Explorer via pacote MSIX "sparse" (ver AppxManifest.xml).
// Cada item chama: powershell.exe -File "<pasta da DLL>\fast-apk.ps1" install|uninstall "<arquivo.apk>"
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <shobjidl.h>
#include <shlwapi.h>
#include <new>
#include <string>

static HMODULE g_module = nullptr;
static LONG g_refs = 0;

// {A3C1F0E2-5B7D-4E8A-9C61-2D4F8B9E1A01}
static const CLSID CLSID_Install = {0xa3c1f0e2, 0x5b7d, 0x4e8a, {0x9c, 0x61, 0x2d, 0x4f, 0x8b, 0x9e, 0x1a, 0x01}};
// {A3C1F0E2-5B7D-4E8A-9C61-2D4F8B9E1A02}
static const CLSID CLSID_Uninstall = {0xa3c1f0e2, 0x5b7d, 0x4e8a, {0x9c, 0x61, 0x2d, 0x4f, 0x8b, 0x9e, 0x1a, 0x02}};

static std::wstring ModuleDir() {
    wchar_t path[MAX_PATH * 2];
    DWORD n = GetModuleFileNameW(g_module, path, ARRAYSIZE(path));
    std::wstring s(path, n);
    size_t p = s.find_last_of(L"\\/");
    return p == std::wstring::npos ? s : s.substr(0, p);
}

static HRESULT DupString(const wchar_t* s, LPWSTR* out) {
    size_t bytes = (wcslen(s) + 1) * sizeof(wchar_t);
    *out = static_cast<LPWSTR>(CoTaskMemAlloc(bytes));
    if (!*out) return E_OUTOFMEMORY;
    memcpy(*out, s, bytes);
    return S_OK;
}

class ApkCommand : public IExplorerCommand {
public:
    explicit ApkCommand(bool install) : m_refs(1), m_install(install) { InterlockedIncrement(&g_refs); }
    virtual ~ApkCommand() { InterlockedDecrement(&g_refs); }

    // IUnknown
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

    // IExplorerCommand
    IFACEMETHODIMP GetTitle(IShellItemArray*, LPWSTR* name) override {
        return DupString(m_install ? L"Instalar via adb" : L"Desinstalar via adb", name);
    }
    IFACEMETHODIMP GetIcon(IShellItemArray*, LPWSTR* icon) override {
        std::wstring p = ModuleDir() + (m_install ? L"\\install.ico" : L"\\uninstall.ico");
        if (GetFileAttributesW(p.c_str()) == INVALID_FILE_ATTRIBUTES) {
            wchar_t sysdir[MAX_PATH];
            GetSystemDirectoryW(sysdir, MAX_PATH);
            p = std::wstring(sysdir) + (m_install ? L"\\imageres.dll,-1028" : L"\\imageres.dll,-5305");
        }
        return DupString(p.c_str(), icon);
    }
    IFACEMETHODIMP GetToolTip(IShellItemArray*, LPWSTR* tip) override { *tip = nullptr; return E_NOTIMPL; }
    IFACEMETHODIMP GetCanonicalName(GUID* guid) override { *guid = m_install ? CLSID_Install : CLSID_Uninstall; return S_OK; }
    IFACEMETHODIMP GetState(IShellItemArray*, BOOL, EXPCMDSTATE* state) override { *state = ECS_ENABLED; return S_OK; }
    IFACEMETHODIMP GetFlags(EXPCMDFLAGS* flags) override { *flags = ECF_DEFAULT; return S_OK; }
    IFACEMETHODIMP EnumSubCommands(IEnumExplorerCommand** e) override { *e = nullptr; return E_NOTIMPL; }

    IFACEMETHODIMP Invoke(IShellItemArray* items, IBindCtx*) override {
        if (!items) return S_OK;
        DWORD count = 0;
        items->GetCount(&count);
        wchar_t sysdir[MAX_PATH];
        GetSystemDirectoryW(sysdir, MAX_PATH);
        std::wstring ps = std::wstring(sysdir) + L"\\WindowsPowerShell\\v1.0\\powershell.exe";
        std::wstring script = ModuleDir() + L"\\fast-apk.ps1";

        for (DWORD i = 0; i < count; i++) {
            IShellItem* item = nullptr;
            if (FAILED(items->GetItemAt(i, &item))) continue;
            LPWSTR path = nullptr;
            if (SUCCEEDED(item->GetDisplayName(SIGDN_FILESYSPATH, &path))) {
                std::wstring cmd = L"\"" + ps + L"\" -NoProfile -ExecutionPolicy Bypass -File \"" + script + L"\" " +
                                   (m_install ? L"install" : L"uninstall") + L" \"" + path + L"\"";
                STARTUPINFOW si = {sizeof(si)};
                PROCESS_INFORMATION pi = {};
                std::wstring buf = cmd;
                if (CreateProcessW(ps.c_str(), &buf[0], nullptr, nullptr, FALSE, CREATE_NEW_CONSOLE, nullptr, nullptr, &si, &pi)) {
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
    LONG m_refs;
    bool m_install;
};

class Factory : public IClassFactory {
public:
    explicit Factory(bool install) : m_refs(1), m_install(install) { InterlockedIncrement(&g_refs); }
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
        ApkCommand* cmd = new (std::nothrow) ApkCommand(m_install);
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
    bool m_install;
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
    bool install;
    if (clsid == CLSID_Install) install = true;
    else if (clsid == CLSID_Uninstall) install = false;
    else return CLASS_E_CLASSNOTAVAILABLE;
    Factory* f = new (std::nothrow) Factory(install);
    if (!f) return E_OUTOFMEMORY;
    HRESULT hr = f->QueryInterface(riid, ppv);
    f->Release();
    return hr;
}

STDAPI DllCanUnloadNow() { return g_refs == 0 ? S_OK : S_FALSE; }

}
