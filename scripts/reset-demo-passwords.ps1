$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
foreach($line in Get-Content -LiteralPath '.env'){if($line -match '^([A-Z0-9_]+)=(.*)$'){[Environment]::SetEnvironmentVariable($Matches[1],$Matches[2],'Process')}}
$env:APP_ENV='local';$env:DEMO_MODE='true';$env:MYSQL_ADDR='127.0.0.1:13306'
& (Join-Path $projectRoot '.local\campusclaw.exe') reset-demo-passwords
if($LASTEXITCODE -ne 0){throw 'Explicit local reset failed'}
