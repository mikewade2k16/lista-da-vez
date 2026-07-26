param(
    [switch]$WithMigrations,
    [switch]$WithWebBuild
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Label,
        [Parameter(Mandatory = $true)][scriptblock]$Action
    )

    Write-Host ""
    Write-Host "==> $Label"
    & $Action
    if ($LASTEXITCODE -ne 0) {
        throw "Falha em: $Label (exit code $LASTEXITCODE)"
    }
}

Push-Location (Join-Path $repoRoot 'back')
try {
    Invoke-Step 'Testes Go dos módulos e da composição' {
        go test -count=1 `
            ./internal/modules/bi `
            ./internal/modules/calendar `
            ./internal/modules/crm/erp `
            ./internal/modules/customerdata `
            ./internal/modules/customerintelligence `
            ./internal/modules/omnichannel `
            ./internal/modules/site `
            ./internal/platform/app `
            ./internal/platform/secretbox
    }

    Invoke-Step 'Vet Go dos módulos e da composição' {
        go vet `
            ./internal/modules/bi `
            ./internal/modules/calendar `
            ./internal/modules/crm/erp `
            ./internal/modules/customerdata `
            ./internal/modules/customerintelligence `
            ./internal/modules/omnichannel `
            ./internal/modules/site `
            ./internal/platform/app `
            ./internal/platform/secretbox
    }

    if ($WithMigrations) {
        if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) {
            throw 'Defina TEST_DATABASE_URL para um PostgreSQL vazio e descartável.'
        }
        Invoke-Step 'Aplicação integral das migrations em banco descartável' {
            go test ./internal/platform/database/... -run TestAllMigrationsApply -count=1 -v
        }
    }
}
finally {
    Pop-Location
}

Push-Location (Join-Path $repoRoot 'web')
try {
    Invoke-Step 'Testes do workspace Customer Intelligence' {
        npm run test -- `
            app/domain/customer-intelligence `
            app/composables/customer-intelligence `
            app/middleware/module-enabled.test.ts
    }

    Invoke-Step 'Lint do escopo Customer Data/Intelligence e integração Omnichannel' {
        npx eslint --no-warn-ignored `
            app/components/customer-intelligence `
            app/composables/customer-intelligence `
            app/domain/customer-data `
            app/domain/customer-intelligence `
            app/domain/omnichannel/channel-client-bindings-api.ts `
            app/composables/omnichannel/useChannelClientBindings.ts `
            app/components/omnichannel/config/ConfigChannelClientBindings.vue `
            app/pages/inteligencia-clientes `
            app/stores/customer-intelligence.ts `
            app/stores/customer-segments.ts `
            app/middleware/module-enabled.global.ts `
            app/middleware/module-enabled.test.ts
    }

    if ($WithWebBuild) {
        Invoke-Step 'Typecheck completo do frontend' {
            npm run typecheck
        }
        Invoke-Step 'Build de produção do frontend' {
            npm run build
        }
    }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host 'Verificação Customer Intelligence concluída.'
