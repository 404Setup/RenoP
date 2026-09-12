$excludeDirNames = [System.Collections.Generic.HashSet[string]]::new(
    [string[]]@('node_modules', 'dist', 'data', 'storage', '.git', '.idea',
        '.gocache', '.gomodcache', 'target', 'bin', 'vendor'),
    [System.StringComparer]::OrdinalIgnoreCase
)
$groups = @{
    '.go' = [long[]]@(0, 0)
    '.js' = [long[]]@(0, 0)
    '.css' = [long[]]@(0, 0)
    '.md' = [long[]]@(0, 0)
}
$root = (Get-Location).ProviderPath

function Read-CounterOutput([string]$Executable, [string[]]$Arguments, [string]$WorkingDirectory) {
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new($Executable)
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.WorkingDirectory = $WorkingDirectory
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.StandardOutputEncoding = [System.Text.UTF8Encoding]::new($false)
    foreach ($argument in $Arguments) { $startInfo.ArgumentList.Add($argument) }
    $process = [System.Diagnostics.Process]::Start($startInfo)
    try {
        $errors = $process.StandardError.ReadToEndAsync()
        $output = $process.StandardOutput.ReadToEnd()
        $process.WaitForExit()
        $null = $errors.GetAwaiter().GetResult()
        if ($process.ExitCode -gt 1) { throw 'Native counting failed' }
        return $output
    } finally {
        $process.Dispose()
    }
}

$usedNative = $false
$rg = Get-Command rg -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -ne $rg -and $null -ne ([System.Diagnostics.ProcessStartInfo]::new()).PSObject.Properties['ArgumentList']) {
    try {
        $common = @('--no-config', '--hidden', '--no-ignore', '--no-messages',
            '--iglob', '*.{go,js,css,md}',
            '--iglob', '!**/{node_modules,dist,data,storage,.git,.idea,.gocache,.gomodcache,target,bin,vendor}/**')
        $output = Read-CounterOutput $rg.Source ($common + @('--stats', '--count-matches', '--include-zero',
            '--null', '-e', '\r|(?-u:[^\r])$|^$', '.')) $root
        [long]$countedFiles = 0
        $position = 0
        while ($position -lt $output.Length) {
            $separator = $output.IndexOf([char]0, $position)
            if ($separator -lt 0) { break }
            $end = $output.IndexOf([char]10, $separator + 1)
            if ($end -lt 0) { throw 'Invalid native counter output' }
            $file = $output.Substring($position, $separator - $position)
            $bucket = $groups[[System.IO.Path]::GetExtension($file)]
            $bucket[0] += [long]::Parse($output.Substring($separator + 1, $end - $separator - 1), [Globalization.CultureInfo]::InvariantCulture)
            $bucket[1]++
            $countedFiles++
            $position = $end + 1
        }
        $searched = [regex]::Match($output.Substring($position), '(?m)^(\d+) files searched\r?$')
        if (-not $searched.Success -or [long]::Parse($searched.Groups[1].Value) -ne $countedFiles) {
            throw 'Native scanner omitted files requiring .NET decoding'
        }
        $usedNative = $true
    } catch {
        foreach ($bucket in $groups.Values) { $bucket[0] = 0; $bucket[1] = 0 }
    }
}

if (-not $usedNative) {
    $directories = [System.Collections.Generic.Stack[System.IO.DirectoryInfo]]::new()
    $directories.Push([System.IO.DirectoryInfo]::new($root))
    while ($directories.Count -gt 0) {
        try {
            foreach ($item in $directories.Pop().EnumerateFileSystemInfos()) {
                if ($item -is [System.IO.DirectoryInfo]) {
                    if (-not $excludeDirNames.Contains($item.Name) -and
                        ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -eq 0) {
                        $directories.Push($item)
                    }
                    continue
                }
                $bucket = $groups[$item.Extension]
                if ($null -eq $bucket) { continue }
                try {
                    $lines = [System.Linq.Enumerable]::LongCount([System.IO.File]::ReadLines($item.FullName))
                    $bucket[0] += $lines
                    $bucket[1]++
                } catch {
                    # Skip this unreadable file and continue with its siblings.
                }
            }
        } catch {
            # Skip unreadable directories.
        }
    }
}

[long]$totalLines = 0
[long]$totalFiles = 0
foreach ($bucket in $groups.Values) { $totalLines += $bucket[0]; $totalFiles += $bucket[1] }
Write-Host "Total: $totalLines ($totalFiles files)"
foreach ($language in @(@('Go', '.go'), @('JS', '.js'), @('CSS', '.css'), @('Markdown', '.md'))) {
    $bucket = $groups[$language[1]]
    Write-Host "$($language[0]): $($bucket[0]) ($($bucket[1]) files)"
}
