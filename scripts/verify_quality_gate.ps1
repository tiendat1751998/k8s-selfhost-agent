<#
.SYNOPSIS
    Quality Gate Verification Script for k8s-selfhost-agent.
    Enforces the Anti-Stupidity Guardrails and Architecture Standards.
#>
param(
    [switch]$SkipBuild = $false
)

$ErrorActionPreference = "Stop"
$failures = 0

Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "   K8S-SELFHOST AUTOMATED QUALITY GATE AUDIT     " -ForegroundColor Cyan
Write-Host "=================================================" -ForegroundColor Cyan

# 1. Check Line Counts (< 500 lines per file)
Write-Host "`n[1/4] Checking file line counts (< 500 lines limit)..." -ForegroundColor Yellow
$monoliths = @()
$codeFiles = Get-ChildItem -Path "internal", "cmd", "frontend-vue/src" -Recurse -Include "*.go", "*.vue", "*.ts" | 
    Where-Object { $_.FullName -notmatch "node_modules|\.git|dist|vendor|_test\.go|tests/" }

foreach ($file in $codeFiles) {
    $lines = (Get-Content $file.FullName | Measure-Object -Line).Lines
    if ($lines -gt 500) {
        $monoliths += [PSCustomObject]@{
            File  = $file.FullName.Replace("$(Get-Location)\", "")
            Lines = $lines
        }
    }
}

if ($monoliths.Count -gt 0) {
    Write-Host "❌ CRITICAL DEFECT: Found $($monoliths.Count) file(s) exceeding 500 lines:" -ForegroundColor Red
    $monoliths | Format-Table -AutoSize
    $failures++
} else {
    Write-Host "✅ PASS: All $($codeFiles.Count) source files strictly under 500 lines." -ForegroundColor Green
}

# 2. Scan for Discarded Errors (_ := or _ = err) in Production Go Code
Write-Host "`n[2/4] Scanning for discarded errors (_ := or _ = err) in production Go code..." -ForegroundColor Yellow
$ignoredErrors = Select-String -Path "internal\*.go", "cmd\*.go" -Pattern '^\s*_\s*[:=]\s*(.+)' -Exclude "*_test.go"
if ($ignoredErrors.Count -gt 0) {
    Write-Host "❌ CRITICAL DEFECT: Found $($ignoredErrors.Count) instance(s) of discarded errors:" -ForegroundColor Red
    $ignoredErrors | Select-Object -First 10 | ForEach-Object {
        Write-Host "   $($_.Path):$($_.LineNumber) -> $($_.Line.Trim())" -ForegroundColor Red
    }
    $failures++
} else {
    Write-Host "✅ PASS: Zero discarded errors found in production Go code." -ForegroundColor Green
}

# 3. Scan for Fake Latency / Mocks (Math.random())
Write-Host "`n[3/4] Scanning for fake mocks / random latency in frontend/backend..." -ForegroundColor Yellow
$fakeMocks = Select-String -Path "frontend-vue\src\*.ts", "frontend-vue\src\*.vue" -Pattern 'Math\.random\(\)' -Exclude "*.spec.ts", "*.test.ts"
if ($fakeMocks.Count -gt 0) {
    Write-Host "❌ DEFECT: Found $($fakeMocks.Count) instance(s) of Math.random():" -ForegroundColor Red
    $fakeMocks | Select-Object -First 5 | ForEach-Object {
        Write-Host "   $($_.Path):$($_.LineNumber) -> $($_.Line.Trim())" -ForegroundColor Red
    }
    $failures++
} else {
    Write-Host "✅ PASS: Zero fake random mocks found in frontend." -ForegroundColor Green
}

# 4. Build and Compile Validation
if (-not $SkipBuild) {
    Write-Host "`n[4/4] Validating Backend Compilation (go build -p 2)..." -ForegroundColor Yellow
    try {
        & go build -p 2 ./cmd/standalone/...
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
        Write-Host "✅ PASS: Standalone backend compiles cleanly." -ForegroundColor Green
    } catch {
        Write-Host "❌ CRITICAL DEFECT: Backend compilation failed!" -ForegroundColor Red
        $failures++
    }

    Write-Host "`nValidating Frontend Build (npm.cmd run build)..." -ForegroundColor Yellow
    try {
        Push-Location "frontend-vue"
        & npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw "npm build failed" }
        Write-Host "✅ PASS: Frontend bundle compiled cleanly." -ForegroundColor Green
    } catch {
        Write-Host "❌ CRITICAL DEFECT: Frontend compilation failed!" -ForegroundColor Red
        $failures++
    } finally {
        Pop-Location
    }
}

Write-Host "`n=================================================" -ForegroundColor Cyan
if ($failures -eq 0) {
    Write-Host "🎉 QUALITY GATE PASSED: ZERO DEFECTS DETECTED." -ForegroundColor Green
    exit 0
} else {
    Write-Host "🚨 QUALITY GATE FAILED: $failures CRITICAL DEFECT(S) DETECTED." -ForegroundColor Red
    exit 1
}
