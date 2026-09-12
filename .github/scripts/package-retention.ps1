# Match actual package directories, including historical short-SHA aliases.
function Test-PackageVersionMatchesRelease {
    param([string]$Version, [object]$Release)
    if ($Version -eq [string]$Release.version) { return $true }
    return $Version -match '^[0-9a-f]{7,40}$' -and
        [string]$Release.commit -match '^[0-9a-f]{40}$' -and
        ([string]$Release.commit).StartsWith($Version, [StringComparison]::OrdinalIgnoreCase)
}

function Get-ObsoletePackageVersions {
    param(
        [object[]]$Releases,
        [string[]]$DirectoryVersions,
        [ValidateSet('nightly', 'stable')][string]$Channel,
        [int]$Retention
    )
    if ($Retention -lt 1) { throw 'Package retention must be positive' }
    $retained = @($Releases | Select-Object -First $Retention)
    $remaining = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($version in $DirectoryVersions) {
        $valid = if ($Channel -eq 'nightly') { $version -match '^[0-9a-f]{7,40}$' }
            else { $version -match '^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$' }
        if (-not $valid) { Write-Host "Skipping non-version directory: $version"; continue }
        if ($Channel -eq 'stable' -and @($Releases | Where-Object { Test-PackageVersionMatchesRelease $version $_ }).Count -eq 0) {
            Write-Host "Skipping untracked stable directory: $version"
            continue
        }
        if (@($retained | Where-Object { Test-PackageVersionMatchesRelease $version $_ }).Count -eq 0) {
            $remaining.Add($version) | Out-Null
        }
    }
    # Known obsolete versions retain publication order. Orphans follow, so rebuilding
    # the 100-entry changelog cannot permanently hide them from cleanup.
    foreach ($release in ($Releases | Select-Object -Skip $Retention)) {
        foreach ($version in @($remaining | Sort-Object)) {
            if (Test-PackageVersionMatchesRelease $version $release) {
                $remaining.Remove($version) | Out-Null
                $version
            }
        }
    }
    $remaining | Sort-Object
}

function Remove-ObsoletePackageVersions {
    param(
        [string[]]$Versions,
        [Parameter(Mandatory)][scriptblock]$DeleteVersion,
        [int]$DeleteLimit = 5,
        [int]$ProbeLimit = 100
    )
    $deleted = 0
    $missing = 0
    $attempted = 0
    $removed = [Collections.Generic.List[string]]::new()
    foreach ($version in $Versions) {
        if ($deleted -ge $DeleteLimit -or $attempted -ge $ProbeLimit) { break }
        $attempted++
        $code = & $DeleteVersion $version
        if ($code -eq 404) { $missing++; $removed.Add($version); continue }
        if ($code -notin @(200, 204)) { throw "DELETE package version $version returned HTTP $code" }
        $deleted++
        $removed.Add($version)
        Write-Host "Deleted obsolete package version: $version"
    }
    Write-Host "Cleanup: $deleted deleted, $missing already missing, $($Versions.Count - $attempted) deferred."
    return $removed.ToArray()
}
