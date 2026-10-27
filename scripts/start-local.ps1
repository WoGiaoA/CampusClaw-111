param([switch]$NoBuild)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
& "$PSScriptRoot\init-env.ps1"
foreach($line in Get-Content -LiteralPath '.env'){if($line -match '^([A-Z0-9_]+)=(.*)$'){[Environment]::SetEnvironmentVariable($Matches[1],$Matches[2],'Process')}}
$mysqlDir=Get-ChildItem -LiteralPath '.local\tools' -Directory -Filter 'mysql-*-winx64' | Select-Object -First 1
if(!$mysqlDir){throw 'Portable MySQL not present. Use Compose after Docker is ready or provide MySQL.'}
$asciiRoot=Join-Path $env:TEMP 'campusclaw-mysql-tools'; if(!(Test-Path (Join-Path $asciiRoot 'bin\mysqld.exe'))){New-Item -ItemType Directory -Force $asciiRoot | Out-Null; Get-ChildItem -LiteralPath $mysqlDir.FullName | Copy-Item -Destination $asciiRoot -Recurse}; $mysqlDir=Get-Item $asciiRoot; $mysqlBin=Join-Path $mysqlDir.FullName 'bin'
$dataPath=Join-Path $env:TEMP 'campusclaw-local-mysql-data'
$basedir=$mysqlDir.FullName.Replace('\','/')
$datadir=$dataPath.Replace('\','/')
if(!(Test-Path (Join-Path $dataPath 'mysql'))){
 New-Item -ItemType Directory -Force -Path $dataPath | Out-Null
 & "$mysqlBin\mysqld.exe" "--basedir=$basedir" "--datadir=$datadir" --initialize-insecure --console 2> '.local\mysql-init.log'
 if($LASTEXITCODE -ne 0){throw 'MySQL initialize failed; see private .local/mysql-init.log'}
}
$already=Get-NetTCPConnection -LocalPort 13306 -State Listen -ErrorAction SilentlyContinue; if($already){$already.OwningProcess | Select-Object -First 1 | Set-Content '.local\mysql.pid'}
if($already){
 $existingMySQL=Get-Process -Id ($already.OwningProcess | Select-Object -First 1)
 if($existingMySQL.Path -ne (Join-Path $mysqlBin 'mysqld.exe')){throw 'Port 13306 belongs to another service; no database will be modified.'}
}
if(!$already){
 $mysqlProcess=Start-Process -FilePath "$mysqlBin\mysqld.exe" -ArgumentList @("--basedir=$basedir","--datadir=$datadir",'--port=13306','--bind-address=127.0.0.1','--mysqlx=OFF','--console') -WindowStyle Hidden -PassThru -RedirectStandardOutput '.local\mysql.stdout.log' -RedirectStandardError '.local\mysql.stderr.log'
 $mysqlProcess.Id | Set-Content '.local\mysql.pid'
 for($attempt=0;$attempt -lt 60;$attempt++){if(Get-NetTCPConnection -LocalPort 13306 -State Listen -ErrorAction SilentlyContinue){break};Start-Sleep -Milliseconds 500}
 if(!(Get-NetTCPConnection -LocalPort 13306 -State Listen -ErrorAction SilentlyContinue)){throw 'Private MySQL did not start; no bootstrap attempted.'}
}
$env:MYSQL_PWD=$env:MYSQL_ROOT_PASSWORD
& "$mysqlBin\mysql.exe" '--host=127.0.0.1' '--port=13306' '--user=root' -e 'SELECT 1' 2> '.local\mysql-connect.log' | Out-Null
if($LASTEXITCODE -ne 0){
 $env:MYSQL_PWD=''
 $sql=@"
ALTER USER 'root'@'localhost' IDENTIFIED BY '$($env:MYSQL_ROOT_PASSWORD)';
CREATE DATABASE IF NOT EXISTS campusclaw CHARACTER SET utf8mb4;
CREATE USER IF NOT EXISTS 'campusclaw'@'localhost' IDENTIFIED BY '$($env:MYSQL_PASSWORD)';
GRANT ALL ON campusclaw.* TO 'campusclaw'@'localhost';
"@
 # SQL contains secrets and stays inside ignored private directory.
 [IO.File]::WriteAllText((Join-Path $projectRoot '.local\bootstrap.sql'),$sql,[Text.UTF8Encoding]::new($false))
 Get-Content -Raw '.local\bootstrap.sql' | & "$mysqlBin\mysql.exe" '--host=127.0.0.1' '--port=13306' '--user=root' 2> '.local\mysql-bootstrap.log'
 if($LASTEXITCODE -ne 0){throw 'Private MySQL bootstrap failed'}
 Remove-Item -LiteralPath (Join-Path $projectRoot '.local\bootstrap.sql')
}
$env:MYSQL_PWD=''
$env:MYSQL_ADDR='127.0.0.1:13306';$env:API_ADDR='127.0.0.1:18081';$env:UPLOAD_DIR=Join-Path $projectRoot '.local\uploads'
$env:APP_ENV='local'; $env:DEMO_MODE='true'
$env:GOPATH=Join-Path $projectRoot '.local\gopath';$env:GOCACHE=Join-Path $projectRoot '.local\gocache'
$go=Join-Path $projectRoot '.local\tools\go\bin\go.exe'
if(!$NoBuild){& $go build -o '.local\campusclaw.exe' ./cmd/api;if($LASTEXITCODE -ne 0){throw 'API build failed'};& $go build -o '.local\devweb.exe' ./cmd/devweb;if($LASTEXITCODE -ne 0){throw 'devweb build failed'}}
foreach($service in @('campusclaw','devweb')){
 $port=if($service -eq 'campusclaw'){18081}else{8080}
 if(!(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)){
  $process=Start-Process -FilePath (Join-Path $projectRoot ".local\$service.exe") -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput ".local\$service.stdout.log" -RedirectStandardError ".local\$service.stderr.log"
  $process.Id | Set-Content ".local\$service.pid"
 }
}
Write-Output 'Local development entry: http://127.0.0.1:8080/login (Go dev proxy, NOT Docker/Nginx).'




