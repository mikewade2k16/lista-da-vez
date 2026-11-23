# Metadados de tempo somente; os argumentos podem conter credenciais e nao sao registrados.
omni_timed_step() {
  local name=$1 start finish status
  shift
  start=$SECONDS
  printf 'OMNI_DEPLOY_STAGE {"name":"%s","status":"started","durationMs":0}\n' "$name"
  if "$@"; then status=0; else status=$?; fi
  finish=$SECONDS
  local outcome=ok
  if [ "$status" -ne 0 ]; then outcome=failed; fi
  printf 'OMNI_DEPLOY_STAGE {"name":"%s","status":"%s","durationMs":%s}\n' "$name" "$outcome" "$(((finish - start) * 1000))"
  return "$status"
}
