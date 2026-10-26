# compact-agent-harness.ps1 - Automated workspace context maintenance (< 60 lines)
$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path "$PSScriptRoot\..").Path
Set-Location $repoRoot

# 1. Rotate decision_log.jsonl if > 25 lines
$decPath = Join-Path $repoRoot ".agents\memory\decision_log.jsonl"
$archPath = Join-Path $repoRoot ".agents\memory\archive\decision_log_2026_08_to_10.jsonl"
if (Test-Path $decPath) {
    $lines = @(Get-Content -LiteralPath $decPath -Encoding UTF8)
    if ($lines.Count -gt 25) {
        $older = $lines[0..($lines.Count - 16)]
        $keep = $lines[($lines.Count - 15)..($lines.Count - 1)]
        $utf8NoBom = New-Object System.Text.UTF8Encoding $false
        [System.IO.File]::AppendAllLines($archPath, $older, $utf8NoBom)
        [System.IO.File]::WriteAllLines($decPath, $keep, $utf8NoBom)
        Write-Host "Rotated $($older.Count) lines to archive. Kept $($keep.Count) active decisions."
    }
}

# 2. Clean inactive session folders in ~/.gemini/antigravity/brain
$brainDir = Join-Path $env:USERPROFILE ".gemini\antigravity\brain"
if (Test-Path $brainDir) {
    Get-ChildItem -LiteralPath $brainDir -Directory | ForEach-Object {
        $folder = $_.FullName
        $isLocked = $false
        Get-ChildItem -LiteralPath $folder -Filter "*.jsonl" -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            try {
                $stream = [System.IO.File]::Open($_.FullName, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
                $stream.Close()
            } catch { $isLocked = $true }
        }
        if (-not $isLocked -and -not (Test-Path (Join-Path $folder "implementation_plan.md"))) {
            Remove-Item -LiteralPath $folder -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# 3. Prune stale git worktrees
git worktree prune

# 4. Summary table of active boot-context sizes
$targetFiles = @("AGENTS.md", ".agents\memory\project_state.md", ".agents\memory\decision_log.jsonl", ".agents\memory\lessons.jsonl") + @(Get-ChildItem -Path ".agents\rules\*.md" | Select-Object -ExpandProperty FullName)
$totalBytes = 0
$rows = foreach ($item in $targetFiles) {
    $fileItem = Get-Item -LiteralPath $item
    $totalBytes += $fileItem.Length
    [PSCustomObject]@{
        File = (Resolve-Path -Relative $fileItem.FullName); Bytes = $fileItem.Length; "KB" = [Math]::Round($fileItem.Length / 1KB, 2)
    }
}
$rows | Format-Table -AutoSize
Write-Host ("=" * 50)
Write-Host ("TOTAL BOOT CONTEXT: {0} bytes ({1:N2} KB)" -f $totalBytes, ($totalBytes / 1KB))
