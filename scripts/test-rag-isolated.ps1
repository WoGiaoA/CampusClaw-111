# Windows PowerShell 5.1. New disposable Qdrant + portable isolated MySQL only.
param([switch]$RealGateway)
$ErrorActionPreference='Stop'
if($RealGateway -and $env:CAMPUSCLAW_REAL_GATEWAY -ne '1'){throw 'Real gateway validation requires CAMPUSCLAW_REAL_GATEWAY=1'}
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
$containerID=$null
$previousURL=$env:QDRANT_TEST_URL
$previousConfirm=$env:QDRANT_TEST_CONFIRM_ISOLATED
$previousFlags=$env:GOFLAGS
try {
 $containerID=(& docker run --rm -d --label campusclaw.purpose=lesson04-isolated-test --tmpfs /qdrant/storage -e 'QDRANT__SERVICE__API_KEY=' --mount "type=bind,source=$projectRoot\deploy\qdrant-entrypoint.sh,target=/qdrant-start.sh,readonly" -p '127.0.0.1::6333' qdrant/qdrant:v1.14.1 /bin/sh /qdrant-start.sh)
 if($LASTEXITCODE -ne 0 -or !$containerID){throw 'Cannot create isolated Qdrant container'}
 $containerID=$containerID.Trim()
 if($containerID -notmatch '^[0-9a-f]{64}$'){throw 'Unexpected container ID; refusing container operations'}
 $binding=(& docker port $containerID 6333/tcp).Trim()
 if($binding -notmatch '^127\.0\.0\.1:(\d+)$'){throw 'Unexpected test port binding'}
 $env:QDRANT_TEST_URL='http://'+$binding
 $env:QDRANT_TEST_CONFIRM_ISOLATED='true'
 $env:GOFLAGS=($previousFlags+' -tags=qdrant').Trim()
 $ready=$false
 for($attempt=0;$attempt -lt 30;$attempt++){
  try{ $response=Invoke-WebRequest -UseBasicParsing -Uri ($env:QDRANT_TEST_URL+'/collections') -TimeoutSec 2; if($response.StatusCode -eq 200){$ready=$true;break} }catch{}
  Start-Sleep -Milliseconds 500
 }
 if(!$ready){throw 'Isolated Qdrant readiness timed out'}
 if ($RealGateway) {
  & "$PSScriptRoot\test-isolated.ps1" -RealGateway
 } else {
  Write-Output 'Real Qdrant v1.14.1 in disposable tmpfs container; embedding/chat use HTTP test doubles, never real keys.'
  & "$PSScriptRoot\test-isolated.ps1"
 }
} finally {
 $env:QDRANT_TEST_URL=$previousURL
 $env:QDRANT_TEST_CONFIRM_ISOLATED=$previousConfirm
 $env:GOFLAGS=$previousFlags
 if($containerID -and $containerID -match '^[0-9a-f]{64}$'){
  $labels=(& docker inspect --format '{{json .Config.Labels}}' $containerID) | ConvertFrom-Json
  $purpose=$labels.'campusclaw.purpose'
  if($LASTEXITCODE -eq 0 -and $purpose -eq 'lesson04-isolated-test'){
   & docker stop $containerID | Out-Null
   Write-Output 'Only this run disposable Qdrant container was stopped and auto-removed.'
  } else {Write-Warning 'Test container identity check failed; left untouched.'}
 }
}
