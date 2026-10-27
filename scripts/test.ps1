param([switch]$Integration)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
$env:GOPATH=Join-Path $projectRoot '.local\gopath'
$env:GOCACHE=Join-Path $projectRoot '.local\gocache'
$goCommand=Get-Command go -ErrorAction SilentlyContinue
$go=if($goCommand){$goCommand.Source}else{Join-Path $projectRoot '.local\tools\go\bin\go.exe'}
if(!(Test-Path -LiteralPath $go)){throw 'Go missing: install Go or provide .local/tools/go/bin/go.exe'}
if(!(Get-Command node -ErrorAction SilentlyContinue)){throw 'Node.js is required for page tests'}
if(!(Test-Path -LiteralPath '.local/ui/node_modules/jsdom')){throw 'Page test dependency missing. Run: npm install --prefix .local/ui --no-save jsdom@30.1.2'}
if($Integration -and !$env:MYSQL_TEST_ADDR){throw '-Integration requires explicit MYSQL_TEST_ADDR and test-only MYSQL_ROOT_PASSWORD. Use scripts/test-isolated.ps1; business .env is never read.'}
$previousTestAddress=$env:MYSQL_TEST_ADDR
if(!$Integration){$env:MYSQL_TEST_ADDR='';Write-Output 'Unit mode: MySQL integration tests will be explicitly SKIP, not PASS.'}
$resultsDir=Join-Path $projectRoot '.local\test-results'
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
try {
 $events=@(& $go test ./... -count=1 -json)
 $testExit=$LASTEXITCODE
 $events | Set-Content -LiteralPath (Join-Path $resultsDir 'go.jsonl') -Encoding UTF8
 $parsed=@($events | ForEach-Object {$_ | ConvertFrom-Json})
 foreach($event in $parsed){if($event.Output){Write-Host $event.Output.TrimEnd()}}
 $top=@($parsed | Where-Object {$_.Test -and $_.Test -notmatch '/' -and $_.Action -in @('pass','skip','fail')})
 $summary=[ordered]@{mode=if($Integration){'isolated-integration'}else{'unit'};passed=@($top | Where-Object Action -eq 'pass' | ForEach-Object Test);skipped=@($top | Where-Object Action -eq 'skip' | ForEach-Object Test);failed=@($top | Where-Object Action -eq 'fail' | ForEach-Object Test)}
 $summary | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $resultsDir 'summary.json') -Encoding UTF8
 Write-Output "Go top-level: $($summary.passed.Count) passed, $($summary.skipped.Count) skipped, $($summary.failed.Count) failed."
 if($testExit -ne 0){throw 'Go tests failed'}
 if($Integration -and $summary.skipped.Count -ne 0){throw 'Integration run had skipped tests; full regression is incomplete'}
 & $go vet ./...
 if($LASTEXITCODE -ne 0){throw 'Go vet failed'}
 & $go build -o (Join-Path $resultsDir 'campusclaw.exe') ./cmd/api
 if($LASTEXITCODE -ne 0){throw 'API build failed'}
 & node --check web/app.js
 if($LASTEXITCODE -ne 0){throw 'Frontend syntax failed'}
 & node --test tests/ui.test.cjs
 if($LASTEXITCODE -ne 0){throw 'UI DOM tests failed'}
 Write-Output 'Build, vet, JS syntax and DOM tests passed. This does not constitute Docker or user acceptance.'
} finally {$env:MYSQL_TEST_ADDR=$previousTestAddress}