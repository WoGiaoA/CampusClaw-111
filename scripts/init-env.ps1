$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$envPath = Join-Path $projectRoot '.env'
if (Test-Path -LiteralPath $envPath) { Write-Output '.env already exists; kept unchanged.'; exit 0 }
$lines = Get-Content -LiteralPath (Join-Path $projectRoot '.env.example')
$secretNames = @('MYSQL_PASSWORD','MYSQL_ROOT_PASSWORD','SESSION_KEY','SEED_TEACHER_A_PASSWORD','SEED_TEACHER_B_PASSWORD','SEED_STUDENT_A1_PASSWORD','SEED_STUDENT_B1_PASSWORD')
$result = foreach ($line in $lines) {
 $parts = $line.Split('=',2)
 if ($parts.Count -eq 2 -and $parts[0] -in $secretNames) {
  $randomBytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($randomBytes) } finally { $rng.Dispose() }
  $parts[0] + '=' + [BitConverter]::ToString($randomBytes).Replace('-','').ToLower()
 } else { $line }
}
[System.IO.File]::WriteAllLines($envPath,$result,[System.Text.UTF8Encoding]::new($false))
# Restrict local generated secrets to the current Windows account.
if ($env:OS -eq 'Windows_NT') { & icacls $envPath /inheritance:r /grant:r "$($env:USERNAME):(F)" | Out-Null }
Write-Output 'Created private .env. Read passwords locally; do not publish it.'
