# Validate initialization using disposable copies; never writes the business .env.
$ErrorActionPreference='Stop'
$taskProjectRoot=Split-Path $PSScriptRoot -Parent
$taskBusinessEnv=Join-Path $taskProjectRoot '.env'
$taskBusinessHash=if(Test-Path -LiteralPath $taskBusinessEnv){(Get-FileHash -LiteralPath $taskBusinessEnv -Algorithm SHA256).Hash}else{$null}
$taskLocalRoot=[IO.Path]::GetFullPath((Join-Path $taskProjectRoot '.local/init-env-check'))
$taskRunRoot=Join-Path $taskLocalRoot ([guid]::NewGuid().ToString('N'))
if(![IO.Path]::GetFullPath($taskRunRoot).StartsWith($taskLocalRoot+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Test path escaped local workspace'}
$taskGenerated=@()
foreach($taskCopyName in @('first','second')){
 $taskCopyRoot=Join-Path $taskRunRoot $taskCopyName
 $taskScripts=Join-Path $taskCopyRoot 'scripts'
 New-Item -ItemType Directory -Force -Path $taskScripts | Out-Null
 Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'init-env.ps1') -Destination (Join-Path $taskScripts 'init-env.ps1')
 Copy-Item -LiteralPath (Join-Path $taskProjectRoot '.env.example') -Destination (Join-Path $taskCopyRoot '.env.example')
 & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskScripts 'init-env.ps1')
 if($LASTEXITCODE -ne 0){throw 'Copied initializer failed'}
 $taskCopyEnv=Join-Path $taskCopyRoot '.env'
 $taskValues=@{}
 foreach($taskLine in Get-Content -LiteralPath $taskCopyEnv){if($taskLine -match '^([A-Z0-9_]+)=(.*)$'){$taskValues[$Matches[1]]=$Matches[2]}}
 foreach($taskName in @('MYSQL_PASSWORD','MYSQL_ROOT_PASSWORD','SESSION_KEY','SEED_TEACHER_A_PASSWORD','SEED_TEACHER_B_PASSWORD','SEED_STUDENT_A1_PASSWORD','SEED_STUDENT_B1_PASSWORD','MCP_SERVER_TOKEN')){
  if($taskValues[$taskName] -notmatch '^[a-f0-9]{64}$'){throw "Invalid generated value: $taskName"}
 }
 if([Convert]::FromBase64String($taskValues['MCP_CREDENTIAL_KEY']).Length -ne 32){throw 'MCP key must decode to 32 bytes'}
 if($taskValues['CHAT_API_KEY'] -or $taskValues['EMBEDDING_API_KEY']){throw 'Initializer must not invent provider credentials'}
 $taskBefore=(Get-FileHash -LiteralPath $taskCopyEnv -Algorithm SHA256).Hash
 & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskScripts 'init-env.ps1')
 if($LASTEXITCODE -ne 0 -or (Get-FileHash -LiteralPath $taskCopyEnv -Algorithm SHA256).Hash -ne $taskBefore){throw 'Existing environment was overwritten'}
 $taskGenerated+=$taskValues
}
foreach($taskName in @('MYSQL_PASSWORD','SESSION_KEY','MCP_SERVER_TOKEN','MCP_CREDENTIAL_KEY')){
 if($taskGenerated[0][$taskName] -eq $taskGenerated[1][$taskName]){throw "Two independent environments reused $taskName"}
}
$taskAfterHash=if(Test-Path -LiteralPath $taskBusinessEnv){(Get-FileHash -LiteralPath $taskBusinessEnv -Algorithm SHA256).Hash}else{$null}
if($taskBusinessHash -ne $taskAfterHash){throw 'Business .env changed'}
Write-Output 'PASS: fresh account/runtime/MCP secrets, independent 32-byte Base64 key, repeat-run preservation, and business .env unchanged.'
Write-Output 'Only disposable copies were used; private generated files remain under ignored .local/init-env-check.'
