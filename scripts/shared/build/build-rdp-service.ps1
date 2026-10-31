#Requires -Version 5.1
<#
.SYNOPSIS
  编译 rdp-service（FreeRDP）。默认由 build-services.ps1（含 pnpm dev:hot）调用。
  无 VS/CMake/OpenSSL 时传 -SkipRdp。
#>
param(
    [ValidateSet('windows')]
    [string]$Platform = 'windows',
    [ValidateSet('x64', 'arm64')]
    [string]$Arch = '',
    [ValidateSet('Release', 'Debug')]
    [string]$Configuration = 'Release'
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

if ([string]::IsNullOrWhiteSpace($Arch)) {
    $Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'x64' }
        'ARM64' { 'arm64' }
        default { 'x64' }
    }
}

$Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
. (Join-Path $Root 'scripts\shared\lib\matrix.ps1')
Assert-NativeMatrix -Platform $Platform -Arch $Arch

$BinDir = Get-ServicesBinDir -RepoRoot $Root -Platform $Platform -Arch $Arch
$BuildDir = Join-Path $Root "services\rdp-service\build\$Platform-$Arch"
function Get-BinaryName {
    param(
        [string]$BaseName,
        [string]$PlatformName
    )
    if ($PlatformName -eq 'windows') {
        return "$BaseName.exe"
    }
    return $BaseName
}

function Ensure-RdpOpenSsl {
    param([string]$RepoRoot)
    $third = Join-Path $RepoRoot 'services\rdp-service\third_party'
    $prefix = Join-Path $third 'openssl'
    $header = Join-Path $prefix 'include\openssl\ssl.h'
    $crypto = Join-Path $prefix 'lib\libcrypto.lib'
    if ((Test-Path $header) -and (Test-Path $crypto)) {
        Write-Host "OpenSSL ready: $prefix"
        return $prefix
    }

    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    if (-not (Test-Path $vswhere)) { throw "vswhere not found: $vswhere" }
    $vsInstall = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
    if (-not $vsInstall) { throw 'Visual Studio C++ toolset not found' }
    $vsDev = Join-Path $vsInstall 'Common7\Tools\VsDevCmd.bat'
    # 托管 runner 的 PATH 里 Strawberry Perl 在 Git perl 前面。
    # Git perl 是 MSYS（$^O=msys），垫片把 PERL5LIB 设成 /d/a/... 并 -MEnsureWin32Paths。
    # Strawberry 会把这条路径放进 @INC，但打不开，Configure 在启动时就退出。
    # $^O 已是 MSWin32 的 perl 直接编 OpenSSL；只有 Git perl 时才加载垫片。
    $gitPerl = 'C:\Program Files\Git\usr\bin\perl.exe'
    $perl = $null
    $useMsysShim = $false
    $perlSeen = @{}
    $perlCandidates = @()
    $pathPerl = Get-Command perl -ErrorAction SilentlyContinue
    if ($pathPerl -and $pathPerl.Source) { $perlCandidates += $pathPerl.Source }
    $perlCandidates += 'C:\Strawberry\perl\bin\perl.exe'
    if ($env:SystemDrive) {
        $perlCandidates += (Join-Path $env:SystemDrive 'Strawberry\perl\bin\perl.exe')
    }
    foreach ($cand in $perlCandidates) {
        if (-not $cand -or $perlSeen.ContainsKey($cand)) { continue }
        $perlSeen[$cand] = $true
        if (-not (Test-Path -LiteralPath $cand)) { continue }
        $perlOs = (& $cand -e 'print $^O' 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and $perlOs -eq 'MSWin32') {
            $perl = $cand
            break
        }
    }
    if (-not $perl) {
        if (-not (Test-Path -LiteralPath $gitPerl)) {
            throw "perl not found: need a Win32 perl (Strawberry) on PATH, or Git for Windows at $gitPerl"
        }
        $perl = $gitPerl
        $useMsysShim = $true
    }

    New-Item -ItemType Directory -Force -Path $third | Out-Null
    $version = '3.5.4'
    $tar = Join-Path $third "openssl-$version.tar.gz"
    $src = Join-Path $third "openssl-$version"
    $url = "https://github.com/openssl/openssl/releases/download/openssl-$version/openssl-$version.tar.gz"
    if (-not (Test-Path $tar) -or (Get-Item $tar).Length -lt 40000000) {
        Write-Host "Downloading OpenSSL $version ..."
        curl.exe -L --fail --retry 3 --output $tar $url
        if ($LASTEXITCODE -ne 0) { throw 'openssl download failed' }
    }
    if (-not (Test-Path (Join-Path $src 'Configure'))) {
        Write-Host "Extracting OpenSSL $version ..."
        tar.exe -xf $tar -C $third
        if ($LASTEXITCODE -ne 0) { throw 'openssl extract failed' }
    }

    $vsArch = 'amd64'
    $target = 'VC-WIN64A'
    if ($Arch -eq 'arm64') {
        $vsArch = 'arm64'
        $target = 'VC-WIN64-ARM'
    }
    # Git 自带的 perl 是 MSYS：$^O=msys，路径是 /e/...，OpenSSL 的 VC-WIN64A 会拒绝。
    # 仅 $useMsysShim 时由 PERL5OPT 加载下面的垫片。Win32 perl 不设置 PERL5LIB / PERL5OPT。
    # 启动时把 $^O 改成 MSWin32，File::Spec 就会给出 E:\ 路径。缺的 Locale::Maketext::Simple 只用于报错文案。
    $perlLib = Join-Path $third 'perl'
    $maketextDir = Join-Path $perlLib 'Locale\Maketext'
    New-Item -ItemType Directory -Force -Path $maketextDir | Out-Null
    $utf8 = New-Object System.Text.UTF8Encoding $false
    $winPaths = @"
package EnsureWin32Paths;
use strict;
use File::Glob ();
BEGIN { `$^O = 'MSWin32'; }
BEGIN {
    my `$orig = \&File::Glob::bsd_glob;
    no warnings 'redefine';
    *File::Glob::bsd_glob = sub {
        my @args = @_;
        if (defined `$args[0]) { `$args[0] =~ s{\\}{/}g; }
        return `$orig->(@args);
    };
}
1;
"@
    $locShim = @"
package Locale::Maketext::Simple;
use strict;
use warnings;
sub import {
    my `$caller = caller;
    no strict 'refs';
    *{"`${caller}::loc"} = \&loc;
}
sub loc {
    my `$fmt = shift;
    `$fmt = '' unless defined `$fmt;
    my `$i = 1;
    foreach my `$arg (@_) {
        my `$n = `$i++;
        `$fmt =~ s/%\Q`$n\E/`$arg/g;
    }
    return `$fmt;
}
1;
"@
    [System.IO.File]::WriteAllText((Join-Path $perlLib 'EnsureWin32Paths.pm'), ($winPaths -replace "(?<!`r)`n", "`r`n"), $utf8)
    [System.IO.File]::WriteAllText((Join-Path $maketextDir 'Simple.pm'), ($locShim -replace "(?<!`r)`n", "`r`n"), $utf8)
    $mmDir = Join-Path $perlLib 'ExtUtils'
    New-Item -ItemType Directory -Force -Path $mmDir | Out-Null
    $makeMaker = @"
package ExtUtils::MakeMaker;
use strict;
use warnings;
package MM;
sub maybe_command {
    my (`$self, `$file) = @_;
    return unless defined `$file && `$file ne '';
    my @ext = split /;/, (`$ENV{PATHEXT} || '.COM;.EXE;.BAT;.CMD');
    if (`$file =~ /\.[^\\\/]+`$/) {
        return `$file if -f `$file;
        return;
    }
    foreach my `$e (@ext) {
        my `$cand = `$file . `$e;
        return `$cand if -f `$cand;
    }
    return `$file if -f `$file;
    return;
}
1;
"@
    [System.IO.File]::WriteAllText((Join-Path $mmDir 'MakeMaker.pm'), ($makeMaker -replace "(?<!`r)`n", "`r`n"), $utf8)
    $podDir = Join-Path $perlLib 'Pod'
    New-Item -ItemType Directory -Force -Path $podDir | Out-Null
    $podUsage = @"
package Pod::Usage;
use strict;
use warnings;
sub import {
    my `$caller = caller;
    no strict 'refs';
    *{"`${caller}::pod2usage"} = sub {
        my %opt = @_;
        my `$code = exists `$opt{-exitval} ? `$opt{-exitval} : 2;
        exit `$code;
    };
}
1;
"@
    [System.IO.File]::WriteAllText((Join-Path $podDir 'Usage.pm'), ($podUsage -replace "(?<!`r)`n", "`r`n"), $utf8)
    $prefixFwd = ($prefix -replace '\\', '/')
    $perlBin = Split-Path $perl -Parent
    $perlLibMsys = $perlLib
    if ($perlLib -match '^([A-Za-z]):\\(.*)$') {
        $perlLibMsys = '/' + $Matches[1].ToLower() + '/' + ($Matches[2] -replace '\\', '/')
    }
    $cmdFile = Join-Path $third 'build-openssl.cmd'
    if ($useMsysShim) {
        Write-Host "Using Git perl: $perl"
        $lines = @(
            '@echo off',
            "call `"$vsDev`" -arch=$vsArch -host_arch=$vsArch",
            'if errorlevel 1 exit /b 1',
            "set `"PATH=%PATH%;$perlBin`"",
            "set `"PERL5LIB=$perlLibMsys`"",
            'set "PERL5OPT=-MEnsureWin32Paths"',
            "cd /d `"$src`"",
            "perl Configure $target no-asm no-shared --prefix=$prefixFwd --openssldir=$prefixFwd/ssl",
            'if errorlevel 1 exit /b 1',
            'nmake',
            'if errorlevel 1 exit /b 1',
            'nmake install_sw',
            'if errorlevel 1 exit /b 1'
        )
    } else {
        # 把这只 perl 放到 PATH 最前，避免 nmake 再叫到 Git 的 MSYS perl。
        # 清掉 PERL5OPT，防止环境里残留 -MEnsureWin32Paths。
        Write-Host "Using Win32 perl: $perl"
        $lines = @(
            '@echo off',
            "call `"$vsDev`" -arch=$vsArch -host_arch=$vsArch",
            'if errorlevel 1 exit /b 1',
            "set `"PATH=$perlBin;%PATH%`"",
            "set `"PERL=$perl`"",
            'set "PERL5OPT="',
            "cd /d `"$src`"",
            "`"%PERL%`" Configure $target no-asm no-shared --prefix=$prefixFwd --openssldir=$prefixFwd/ssl",
            'if errorlevel 1 exit /b 1',
            'nmake',
            'if errorlevel 1 exit /b 1',
            'nmake install_sw',
            'if errorlevel 1 exit /b 1'
        )
    }
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($cmdFile, (($lines -join "`r`n") + "`r`n"), $utf8)
    Write-Host "Building OpenSSL $version (static, no-asm) ..."
    # 编译日志必须离开函数的成功输出流。调用方会把返回值拼进 -DOPENSSL_ROOT_DIR；
    # nmake 的标准输出一旦混进去，cmake 命令行就会超过 Windows 上限，
    # CreateProcess 报 “The filename or extension is too long”。
    # 重定向放在 cmd 内部。PowerShell 的 *> 仍会把 stderr 收成 NativeCommandError，
    # 在 $ErrorActionPreference = Stop 时空白行也会让脚本直接退出。
    $buildLog = Join-Path $third 'build-openssl.log'
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        cmd.exe /d /s /c "`"`"$cmdFile`" > `"$buildLog`" 2>&1`""
        $buildCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $prevEap
    }
    if (Test-Path -LiteralPath $buildLog) {
        Get-Content -LiteralPath $buildLog | ForEach-Object { Write-Host $_ }
    }
    if ($buildCode -ne 0) { throw 'openssl build failed' }
    if (-not (Test-Path $crypto)) { throw "openssl library missing: $crypto" }
    return $prefix
}

$opensslPrefix = Ensure-RdpOpenSsl -RepoRoot $Root
if ($opensslPrefix -is [System.Array]) {
    throw 'openssl prefix was polluted by build log output'
}
$exeName = Get-BinaryName -PlatformName $Platform -BaseName 'ensis-rdp-service'
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null

$cmakeArgs = @(
    '-S', (Join-Path $Root 'services\rdp-service'),
    '-B', $BuildDir,
    "-DCMAKE_BUILD_TYPE=$Configuration",
    "-DOPENSSL_ROOT_DIR=$opensslPrefix",
    '-DOPENSSL_USE_STATIC_LIBS=ON',
    '-DENSIOPS_BUILD_TESTS=OFF'
)
if ($Platform -eq 'windows') {
    $cmakeArgs += @('-G', 'Visual Studio 17 2022', '-A', $(if ($Arch -eq 'arm64') { 'ARM64' } else { 'x64' }))
}

Write-Host "Configuring rdp-service ($Platform-$Arch)..."
cmake @cmakeArgs
if ($LASTEXITCODE -ne 0) { throw 'cmake configure failed' }

cmake --build $BuildDir --config $Configuration --parallel
if ($LASTEXITCODE -ne 0) { throw 'cmake build failed' }

$built = Get-ChildItem -Path $BuildDir -Recurse -Filter $exeName -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -notmatch '\\CMakeFiles\\' } |
    Sort-Object LastWriteTimeUtc -Descending |
    Select-Object -First 1
if (-not $built) {
    throw "built binary not found: $exeName"
}
$dest = Join-Path $BinDir $exeName
Copy-Item -Force $built.FullName $dest
Write-Host "Output: $dest"

$builtDir = Split-Path -Parent $built.FullName
Get-ChildItem -Path $builtDir -Filter '*.dll' -ErrorAction SilentlyContinue | ForEach-Object {
    Copy-Item -Force $_.FullName (Join-Path $BinDir $_.Name)
    Write-Host "DLL: $($_.Name)"
}

if ($Platform -eq 'windows' -and $Arch -eq (Get-MatrixHostArch)) {
    $legacy = Join-Path $Root 'services\bin'
    New-Item -ItemType Directory -Force -Path $legacy | Out-Null
    Copy-Item -Force $dest (Join-Path $legacy $exeName)
    Get-ChildItem -Path $builtDir -Filter '*.dll' -ErrorAction SilentlyContinue | ForEach-Object {
        Copy-Item -Force $_.FullName (Join-Path $legacy $_.Name)
    }
    Write-Host "Legacy: $(Join-Path $legacy $exeName)"
}

Write-Host 'rdp-service build ok'
