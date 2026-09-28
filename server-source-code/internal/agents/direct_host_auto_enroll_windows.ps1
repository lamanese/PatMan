# PatchMon Direct Host Auto-Enrollment Script for Windows
# The server prepends $env:PATCHMON_URL, $env:AUTO_ENROLLMENT_KEY,
# $env:AUTO_ENROLLMENT_SECRET, $env:PATCHMON_IGNORE_SSL and $env:FORCE_INSTALL.
#
# Usage (elevated PowerShell, "Run as Administrator"):
#   irm "https://patchmon.example.com/api/v1/auto-enrollment/script?type=direct-host-windows&token_key=KEY&token_secret=SECRET" | iex
#
# Optional custom name, set before running:
#   $env:FRIENDLY_NAME = "My Server"
#
# The script enrolls this machine with the token (hostname as name, machine
# GUID for identity), then runs the regular Windows agent installer with the
# credentials it received.

$ErrorActionPreference = "Stop"

$ServerURL     = $env:PATCHMON_URL
$TokenKey      = $env:AUTO_ENROLLMENT_KEY
$TokenSecret   = $env:AUTO_ENROLLMENT_SECRET
$SkipSslVerify = ($env:PATCHMON_IGNORE_SSL -eq "true" -or $env:PATCHMON_IGNORE_SSL -eq "1")
$ForceInstall  = ($env:FORCE_INSTALL -eq "true" -or $env:FORCE_INSTALL -eq "1")
$AgentExe      = "C:\Program Files\PatchMon\patchmon-agent.exe"

function Write-Info($m)    { Write-Host "[INFO] $m" -ForegroundColor Green }
function Write-Warn($m)    { Write-Host "[WARN] $m" -ForegroundColor Yellow }
function Write-Success($m) { Write-Host "[SUCCESS] $m" -ForegroundColor Green }
# throw (not exit): with "irm | iex" an exit would close the operator's console.
function Fail($m) { Write-Host "[ERROR] $m" -ForegroundColor Red; throw "PatchMon auto-enrollment failed: $m" }

Write-Host ""
Write-Host "PatchMon Direct Host Auto-Enrollment (Windows)" -ForegroundColor Cyan
Write-Host ""

# ===== VALIDATION =====
Write-Info "Validating configuration..."
if (-not $ServerURL -or -not $TokenKey -or -not $TokenSecret) {
    Fail "PATCHMON_URL, AUTO_ENROLLMENT_KEY and AUTO_ENROLLMENT_SECRET must be set"
}
$identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Fail "This script must run in an elevated PowerShell (Run as Administrator)"
}
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
if ($SkipSslVerify) {
    [Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
}
Write-Info "PatchMon Server: $ServerURL"

# ===== ALREADY ENROLLED? =====
if (Test-Path $AgentExe) {
    $pingOk = $false
    try { & $AgentExe ping *> $null; $pingOk = ($LASTEXITCODE -eq 0) } catch { $pingOk = $false }
    if ($pingOk) {
        Write-Success "Host already enrolled and agent ping successful - nothing to do"
        return
    }
    Write-Warn "Agent present but ping failed - will reinstall"
}

# ===== GATHER HOST INFORMATION =====
$hostname = $env:COMPUTERNAME
$friendlyName = if ($env:FRIENDLY_NAME) { $env:FRIENDLY_NAME } else { $hostname }
$machineId = ""
try { $machineId = (Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Cryptography" -Name MachineGuid -ErrorAction Stop).MachineGuid } catch { $machineId = "" }
$osInfo = "Windows"
try { $osInfo = (Get-CimInstance Win32_OperatingSystem -ErrorAction Stop).Caption } catch { }
$ipAddress = "unknown"
try {
    $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop | Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } | Select-Object -First 1
    if ($ip) { $ipAddress = $ip.IPAddress }
} catch { }
$architecture = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

Write-Info "Hostname: $hostname"
Write-Info "Friendly Name: $friendlyName"
Write-Info "IP Address: $ipAddress"
Write-Info "OS: $osInfo"
Write-Info "Architecture: $architecture"
if ($machineId) { Write-Info "Machine ID: $($machineId.Substring(0, [Math]::Min(16, $machineId.Length)))..." } else { Write-Info "Machine ID: (not available)" }
Write-Host ""

# ===== ENROLL HOST =====
Write-Info "Enrolling $friendlyName in PatchMon..."
$payload = @{
    friendly_name = $friendlyName
    metadata      = @{ hostname = $hostname; ip_address = $ipAddress; os_info = $osInfo; architecture = $architecture; platform = "windows" }
}
if ($machineId) { $payload.machine_id = $machineId }
$headers = @{ "X-Auto-Enrollment-Key" = $TokenKey; "X-Auto-Enrollment-Secret" = $TokenSecret }
$enrollUrl = "$ServerURL/api/v1/auto-enrollment/enroll"
try {
    $response = Invoke-RestMethod -Uri $enrollUrl -Method Post -Headers $headers -Body ($payload | ConvertTo-Json -Depth 4) -ContentType "application/json" -UseBasicParsing
} catch {
    $status = ""
    $detail = $_.Exception.Message
    if ($_.Exception.Response) {
        try { $status = [int]$_.Exception.Response.StatusCode } catch { }
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            if ($stream) { $reader = New-Object IO.StreamReader($stream); $detail = $reader.ReadToEnd() }
        } catch { }
    }
    Fail "Failed to enroll $friendlyName - HTTP $status $detail"
}
$apiId  = $response.host.api_id
$apiKey = $response.host.api_key
if (-not $apiId -or -not $apiKey) { Fail "Failed to parse API credentials from response" }
if ($response.host.friendly_name -and $response.host.friendly_name -ne $friendlyName) {
    Write-Warn "Name '$friendlyName' was taken; enrolled as '$($response.host.friendly_name)'"
}
Write-Success "Host enrolled successfully: $apiId"
Write-Host ""

# ===== INSTALL AGENT =====
Write-Info "Installing PatchMon agent..."
$installUrl = "$ServerURL/api/v1/hosts/install?os=windows&arch=$architecture"
if ($ForceInstall) { $installUrl += "&force=true" }
$installHeaders = @{ "X-API-ID" = $apiId; "X-API-KEY" = $apiKey }
$installScript = (Invoke-WebRequest -Uri $installUrl -Headers $installHeaders -UseBasicParsing).Content
$installPath = Join-Path $env:TEMP "patchmon-install.ps1"
$installScript | Out-File -FilePath $installPath -Encoding utf8
# Run the installer as its own process: its exit codes stay out of this console.
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installPath
$installExit = $LASTEXITCODE
Remove-Item -Path $installPath -Force -ErrorAction SilentlyContinue
if ($installExit -ne 0) { Fail "Failed to install agent (exit: $installExit)" }
Write-Success "Agent installed successfully"
Write-Host ""
Write-Success "Auto-enrollment complete!"
