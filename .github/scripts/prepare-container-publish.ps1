[CmdletBinding()]
param(
    [string]$BaseUrl = 'https://mvnc.pkg.one',
    [string]$Repository = 'oci',
    [string]$Image = 'renop',
    [string]$OutputFile = $env:GITHUB_OUTPUT
)
$ErrorActionPreference = 'Stop'
$token = $env:RENOP_PUBLISH_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) { throw 'RENOP_PUBLISH_TOKEN is not set' }
if ($Repository -notmatch '^[a-z0-9_-]+$' -or $Image -notmatch '^[a-z0-9_-]+$') {
    throw 'Container publication requires single-component repository and image names'
}
$base = [Uri]$BaseUrl.TrimEnd('/')
if ($base.Scheme -ne 'https' -and -not ($base.Scheme -eq 'http' -and $base.IsLoopback)) {
    throw 'Registry credentials require HTTPS'
}
$handler = [Net.Http.SocketsHttpHandler]::new()
$handler.AllowAutoRedirect = $false
$client = [Net.Http.HttpClient]::new($handler)
$client.Timeout = [TimeSpan]::FromSeconds(30)
$client.MaxResponseContentBufferSize = 65536
$client.DefaultRequestHeaders.Authorization = [Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer', $token)

function Invoke-RegistryControl {
    param([string]$Method, [string]$Path, [object]$Body = $null)
    $request = [Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::new($Method), "$($base.AbsoluteUri.TrimEnd('/'))$Path")
    $response = $null
    try {
        if ($null -ne $Body) {
            $request.Content = [Net.Http.StringContent]::new(($Body | ConvertTo-Json -Compress), [Text.Encoding]::UTF8, 'application/json')
        }
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $content = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        return @{Status = [int]$response.StatusCode; Content = $content}
    } finally {
        if ($null -ne $response) { $response.Dispose() }
        $request.Dispose()
    }
}

try {
    $path = "/api/docker/repositories/$Repository/images/$Image"
    $existing = Invoke-RegistryControl GET $path
    if ($existing.Status -eq 404) {
        $created = Invoke-RegistryControl POST "/api/docker/repositories/$Repository/images" @{image = $Image; private = $false}
        if ($created.Status -eq 202) { throw 'Container image creation is pending review; approve it before publishing' }
        if ($created.Status -notin @(200, 201, 409)) { throw "Container image creation returned HTTP $($created.Status)" }
        $existing = Invoke-RegistryControl GET $path
    }
    if ($existing.Status -ne 200) { throw "Container image lookup returned HTTP $($existing.Status)" }

    # Resolve the account through the registry's authenticated token response. This
    # does not require adding account:read to the existing publishing API token.
    $scope = [Uri]::EscapeDataString("repository:${Repository}/${Image}:pull,push")
    $grant = Invoke-RegistryControl GET "/v2/token?service=$($base.Authority)&scope=$scope"
    if ($grant.Status -ne 200) { throw "Registry token request returned HTTP $($grant.Status)" }
    $jwt = [string](($grant.Content | ConvertFrom-Json).token)
    $parts = $jwt.Split('.')
    if ($parts.Count -ne 3) { throw 'Registry returned an invalid token' }
    $payload = $parts[1].Replace('-', '+').Replace('_', '/')
    $payload = $payload.PadRight($payload.Length + (4 - $payload.Length % 4) % 4, '=')
    $claims = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($payload)) | ConvertFrom-Json
    $username = [string]$claims.sub
    if ($username -notmatch '^[a-zA-Z0-9_.-]{1,64}$' -or $username -eq 'guest') { throw 'Registry did not authenticate the publishing account' }
    $access = @($claims.access | Where-Object { $_.type -eq 'repository' -and $_.name -eq "$Repository/$Image" })
    if ($access.Count -ne 1 -or 'push' -notin $access[0].actions -or 'pull' -notin $access[0].actions) {
        throw 'The existing API token does not have pull and push access to the container image'
    }
    if ($OutputFile) { "username=$username" >> $OutputFile }
    Write-Host "Container publication is ready for $($base.Authority)/$Repository/$Image"
} finally {
    $client.Dispose()
}
