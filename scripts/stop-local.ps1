$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
foreach($name in @('devweb','campusclaw','mysql')){
 $pidPath=Join-Path $projectRoot ".local\$name.pid"
 if(Test-Path $pidPath){
  $processId=[int](Get-Content $pidPath)
  $process=Get-Process -Id $processId -ErrorAction SilentlyContinue
  $expected=if($name -eq 'mysql'){'mysqld'}else{$name}
  if($process -and $process.ProcessName -eq $expected){Stop-Process -Id $processId}
  Remove-Item -LiteralPath $pidPath
 }
}
Write-Output 'Local processes stopped; private database and uploads preserved.'
