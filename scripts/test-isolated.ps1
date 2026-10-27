# Run from Windows PowerShell 5.1 or newer. Never reads .env or starts Docker.
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
$mysqlTools=Get-ChildItem -LiteralPath '.local\tools' -Directory -Filter 'mysql-*-winx64' | Select-Object -First 1
if(!$mysqlTools){throw 'Portable MySQL tools missing under .local/tools; no service or business data was touched.'}
$mysqld=Join-Path $mysqlTools.FullName 'bin\mysqld.exe'
$mysqladmin=Join-Path $mysqlTools.FullName 'bin\mysqladmin.exe'
$runRoot=Join-Path $projectRoot ('.local\regression-db\'+[guid]::NewGuid().ToString('N'))
$dataRoot=Join-Path $runRoot 'data'
New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
$randomBytes=New-Object byte[] 32
$rng=[Security.Cryptography.RandomNumberGenerator]::Create()
try{$rng.GetBytes($randomBytes)}finally{$rng.Dispose()}
$testPassword=[BitConverter]::ToString($randomBytes).Replace('-','').ToLower()
$bootstrap=Join-Path $runRoot 'bootstrap.sql'
[IO.File]::WriteAllText($bootstrap,"ALTER USER 'root'@'localhost' IDENTIFIED BY '$testPassword';",[Text.UTF8Encoding]::new($false))
$previousAddress=$env:MYSQL_TEST_ADDR
$previousPassword=$env:MYSQL_ROOT_PASSWORD
$previousClientPassword=$env:MYSQL_PWD
$serverProcess=$null
$testFailure=$null
try {
 $baseArg='--basedir="'+$mysqlTools.FullName+'"'
 $dataArg='--datadir="'+$dataRoot+'"'
 $init=Start-Process -FilePath $mysqld -ArgumentList @($baseArg,$dataArg,'--initialize-insecure','--console') -WindowStyle Hidden -PassThru -Wait -RedirectStandardOutput (Join-Path $runRoot 'init.stdout.log') -RedirectStandardError (Join-Path $runRoot 'init.stderr.log')
 if($init.ExitCode -ne 0){throw 'Isolated MySQL initialization failed; see private .local/regression-db logs'}
 $listener=New-Object System.Net.Sockets.TcpListener([Net.IPAddress]::Loopback,0)
 $listener.Start();$testPort=$listener.LocalEndpoint.Port;$listener.Stop()
 $initArg='--init-file="'+$bootstrap+'"'
 $serverProcess=Start-Process -FilePath $mysqld -ArgumentList @($baseArg,$dataArg,$initArg,"--port=$testPort",'--bind-address=127.0.0.1','--mysqlx=OFF','--skip-log-bin','--innodb-buffer-pool-size=64M','--console') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runRoot 'server.stdout.log') -RedirectStandardError (Join-Path $runRoot 'server.stderr.log')
 $env:MYSQL_PWD=$testPassword
 $ready=$false
 for($attempt=0;$attempt -lt 100;$attempt++){
  $serverProcess.Refresh();if($serverProcess.HasExited){throw 'Isolated MySQL exited before readiness; see private logs'}
  $probe=Start-Process -FilePath $mysqladmin -ArgumentList @('--protocol=tcp','--host=127.0.0.1',"--port=$testPort",'--user=root','--connect-timeout=1','ping','--silent') -WindowStyle Hidden -PassThru -Wait -RedirectStandardOutput (Join-Path $runRoot 'ping.stdout.log') -RedirectStandardError (Join-Path $runRoot 'ping.stderr.log')
  if($probe.ExitCode -eq 0){$ready=$true;break}
  Start-Sleep -Milliseconds 250
 }
 if(!$ready){throw 'Isolated MySQL readiness timed out'}
 Remove-Item -LiteralPath $bootstrap
 $env:MYSQL_TEST_ADDR="127.0.0.1:$testPort"
 $env:MYSQL_ROOT_PASSWORD=$testPassword
 Write-Output 'Running real HTTP/DB tests against a new isolated MySQL data directory; no business database or Docker is used.'
 & "$PSScriptRoot\test.ps1" -Integration
} catch {$testFailure=$_} finally {
 if($serverProcess){
  $serverProcess.Refresh()
  if(!$serverProcess.HasExited){
   $shutdown=Start-Process -FilePath $mysqladmin -ArgumentList @('--protocol=tcp','--host=127.0.0.1',"--port=$testPort",'--user=root','--connect-timeout=2','shutdown') -WindowStyle Hidden -PassThru -Wait -RedirectStandardOutput (Join-Path $runRoot 'shutdown.stdout.log') -RedirectStandardError (Join-Path $runRoot 'shutdown.stderr.log')
   if(!$serverProcess.WaitForExit(10000)){Stop-Process -Id $serverProcess.Id}
  }
 }
 if(Test-Path -LiteralPath $bootstrap){Remove-Item -LiteralPath $bootstrap}
 $env:MYSQL_TEST_ADDR=$previousAddress
 $env:MYSQL_ROOT_PASSWORD=$previousPassword
 $env:MYSQL_PWD=$previousClientPassword
 Write-Output 'This run MySQL process stopped. Isolated test files retained under ignored .local/regression-db.'
}
if($testFailure){throw $testFailure}