# t
# PowerShell script to test Stream Management Service on Windows

Write-Host "🔍 Testing Stream Management Service on Windows..." -ForegroundColor Cyan

# Test 1: Health check
Write-Host "`n1. Testing health endpoint..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8084/health" -Method GET
    Write-Host "✅ Health check passed:" -ForegroundColor Green
    $health | ConvertTo-Json
} catch {
    Write-Host "❌ Health check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 2: RTMP health
Write-Host "`n2. Testing RTMP health endpoint..." -ForegroundColor Yellow
try {
    $rtmpHealth = Invoke-RestMethod -Uri "http://localhost:8084/rtmp/health" -Method GET
    Write-Host "✅ RTMP health check passed:" -ForegroundColor Green
    $rtmpHealth | ConvertTo-Json
} catch {
    Write-Host "❌ RTMP health check failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 3: Simulate SRS auth callback
Write-Host "`n3. Testing RTMP auth endpoint (simulating SRS callback)..." -ForegroundColor Yellow
try {
    $authBody = @{
        name = "cCJ-XUxQ8nHtD-FonqifVshSYe2Jm8kIux-4iuuqhzs"
        addr = "172.20.0.1"
        app = "live"
    }

    $authResponse = Invoke-RestMethod -Uri "http://localhost:8084/rtmp/auth" -Method POST -Body $authBody -ContentType "application/x-www-form-urlencoded"
    Write-Host "✅ Auth test passed:" -ForegroundColor Green
    $authResponse | ConvertTo-Json
} catch {
    Write-Host "❌ Auth test failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 4: Check if port 8084 is listening
Write-Host "`n4. Checking if service is listening on port 8084..." -ForegroundColor Yellow
$listening = Get-NetTCPConnection -LocalPort 8084 -State Listen -ErrorAction SilentlyContinue
if ($listening) {
    Write-Host "✅ Service is listening on port 8084" -ForegroundColor Green
    $listening | Select-Object LocalAddress, LocalPort, State, OwningProcess
} else {
    Write-Host "❌ No service listening on port 8084" -ForegroundColor Red
    Write-Host "Make sure your Stream Management Service is running!" -ForegroundColor Yellow
}

# Test 5: Test Docker connectivity
Write-Host "`n5. Testing Docker host.docker.internal connectivity..." -ForegroundColor Yellow
try {
    $dockerHost = Test-NetConnection -ComputerName "host.docker.internal" -Port 8084 -WarningAction SilentlyContinue
    if ($dockerHost.TcpTestSucceeded) {
        Write-Host "✅ host.docker.internal:8084 is reachable from Docker" -ForegroundColor Green
    } else {
        Write-Host "❌ host.docker.internal:8084 is NOT reachable from Docker" -ForegroundColor Red
    }
} catch {
    Write-Host "⚠️ Could not test Docker connectivity: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Test 6: Get Windows host IP for reference
Write-Host "`n6. Your Windows host IP addresses:" -ForegroundColor Yellow
Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin Dhcp,Manual |
        Where-Object {$_.IPAddress -ne "127.0.0.1"} |
        Select-Object IPAddress, InterfaceAlias |
        ForEach-Object { Write-Host "   $($_.IPAddress) ($($_.InterfaceAlias))" -ForegroundColor Cyan }

Write-Host "`n✅ Test completed!"
