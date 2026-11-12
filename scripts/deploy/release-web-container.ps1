function Get-ReleaseWebContainer {
  $containerIds = @(& docker compose ps -q web)
  if ($LASTEXITCODE -ne 0) {
    throw 'Nao foi possivel consultar o container web local. Verifique o Docker Desktop e o Compose antes de tentar novamente.'
  }
  $containerIds = @($containerIds | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim() })
  if ($containerIds.Count -ne 1) {
    throw 'O container web local nao esta em execucao. No Mac, inicie a stack com npm run dev:mac:up e aguarde o web ficar pronto antes de tentar novamente.'
  }
  return $containerIds[0]
}
