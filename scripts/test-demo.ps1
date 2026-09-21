param(
    [string]$BrowserPath = 'C:\Program Files\Google\Chrome\Application\chrome.exe',
    [switch]$Headful
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $BrowserPath -PathType Leaf)) {
    throw "Browser not found: $BrowserPath"
}
$previousBrowser = $env:DRISSIONPAGE_BROWSER
$previousHeadful = $env:DRISSIONPAGE_HEADFUL
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    $env:DRISSIONPAGE_BROWSER = $BrowserPath
    $env:DRISSIONPAGE_HEADFUL = if ($Headful) { '1' } else { '0' }
    & go test -count=1 -timeout 2m -v -run '^TestAsyncHTMLDemo$' .
    $resultCode = $LASTEXITCODE
} finally {
    $env:DRISSIONPAGE_BROWSER = $previousBrowser
    $env:DRISSIONPAGE_HEADFUL = $previousHeadful
    Pop-Location
}
exit $resultCode
