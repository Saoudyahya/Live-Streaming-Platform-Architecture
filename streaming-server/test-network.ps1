# streaming-server/test-network.ps1
# PowerShell script to test network connectivity

Write-Host "🔍 Testing network connectivity from SRS container to Windows host..." -ForegroundColor Cyan

# Check if Docker is running
try {
    $containers = docker ps -q -f name=streaming-server-pro
    if (-not $containers) {
        Write-Host "❌ SRS container not found. Starting it..." -ForegroundColor Red
        docker-compose up -d
        Start-Sleep 5
        $containers = docker ps -q -f name=streaming-server-pro
    }

    if (-not $containers) {
        Write-Host "❌ Failed to start SRS container" -ForegroundColor Red
        exit 1
    }

    Write-Host "📋 Container found: $containers" -ForegroundColor Green
} catch {
    Write-Host "❌ Docker is not running or accessible" -ForegroundColor Red
    exit 1
}

Write-Host "`n🔗 Testing connectivity options:" -ForegroundColor Yellow

# Initialize working URL variable
$workingUrl = $null

# Test 1: host.docker.internal (should work on Docker Desktop)
Write-Host "1. Testing host.docker.internal:8084..." -ForegroundColor White
try {
    $result1 = docker exec $containers sh -c "wget -q -O- --timeout=5 http://host.docker.internal:8084/health || echo 'FAILED'"
    if ($result1 -and $result1 -ne "FAILED" -and $result1.Trim() -ne "") {
        Write-Host "✅ host.docker.internal works!" -ForegroundColor Green
        $workingUrl = "host.docker.internal:8084"
    } else {
        Write-Host "❌ host.docker.internal failed" -ForegroundColor Red
    }
} catch {
    Write-Host "❌ host.docker.internal failed with error: $_" -ForegroundColor Red
}

# Test 2: Your Windows IP
try {
    $windowsIp = (Get-NetIPConfiguration | Where-Object {$_.IPv4DefaultGateway -ne $null -and $_.NetAdapter.Status -ne "Disconnected"}).IPv4Address.IPAddress | Select-Object -First 1
    Write-Host "`n2. Testing Windows IP ($windowsIp):8084..." -ForegroundColor White

    $result2 = docker exec $containers sh -c "wget -q -O- --timeout=5 http://${windowsIp}:8084/health || echo 'FAILED'"
    if ($result2 -and $result2 -ne "FAILED" -and $result2.Trim() -ne "") {
        Write-Host "✅ Windows IP ($windowsIp) works!" -ForegroundColor Green
        if (-not $workingUrl) { $workingUrl = "${windowsIp}:8084" }
    } else {
        Write-Host "❌ Windows IP ($windowsIp) failed" -ForegroundColor Red
    }
} catch {
    Write-Host "❌ Windows IP test failed with error: $_" -ForegroundColor Red
}

# Test 3: Docker bridge (usually doesn't work on Windows)
Write-Host "`n3. Testing Docker bridge (172.17.0.1):8084..." -ForegroundColor White
try {
    $result3 = docker exec $containers sh -c "wget -q -O- --timeout=5 http://172.17.0.1:8084/health || echo 'FAILED'"
    if ($result3 -and $result3 -ne "FAILED" -and $result3.Trim() -ne "") {
        Write-Host "✅ Docker bridge works!" -ForegroundColor Green
        if (-not $workingUrl) { $workingUrl = "172.17.0.1:8084" }
    } else {
        Write-Host "❌ Docker bridge failed (expected on Windows)" -ForegroundColor Red
    }
} catch {
    Write-Host "❌ Docker bridge failed with error: $_" -ForegroundColor Red
}

# Test 4: Gateway IP (alternative approach)
Write-Host "`n4. Testing Gateway IP..." -ForegroundColor White
try {
    $gatewayInfo = docker exec $containers sh -c "ip route show default | grep default | awk '{print \$3}'"
    if ($gatewayInfo) {
        Write-Host "   Gateway IP found: $gatewayInfo" -ForegroundColor Gray
        $result4 = docker exec $containers sh -c "wget -q -O- --timeout=5 http://${gatewayInfo}:8084/health || echo 'FAILED'"
        if ($result4 -and $result4 -ne "FAILED" -and $result4.Trim() -ne "") {
            Write-Host "✅ Gateway IP works!" -ForegroundColor Green
            if (-not $workingUrl) { $workingUrl = "${gatewayInfo}:8084" }
        } else {
            Write-Host "❌ Gateway IP failed" -ForegroundColor Red
        }
    }
} catch {
    Write-Host "❌ Gateway IP test failed with error: $_" -ForegroundColor Red
}

Write-Host "`n🔍 Container network information:" -ForegroundColor Yellow
try {
    Write-Host "Hosts file entries:" -ForegroundColor Gray
    docker exec $containers sh -c "grep -E '(host\.docker\.internal|gateway)' /etc/hosts"

    Write-Host "`nDefault route:" -ForegroundColor Gray
    docker exec $containers sh -c "ip route show default"

    Write-Host "`nContainer IP:" -ForegroundColor Gray
    docker exec $containers sh -c "hostname -i"
} catch {
    Write-Host "❌ Could not retrieve network info" -ForegroundColor Red
}

Write-Host "`n📝 Next steps:" -ForegroundColor Cyan
if ($workingUrl) {
    Write-Host "✅ Use this URL in your SRS config: $workingUrl" -ForegroundColor Green
    Write-Host "   Update streaming-server/config/srs.config with:" -ForegroundColor White
    Write-Host "   on_publish      http://$workingUrl/rtmp/auth;" -ForegroundColor Gray
    Write-Host "   on_publish_done http://$workingUrl/rtmp/started;" -ForegroundColor Gray
    Write-Host "   on_unpublish    http://$workingUrl/rtmp/ended;" -ForegroundColor Gray
} else {
    Write-Host "❌ No working URL found. Check Windows Firewall and Docker settings" -ForegroundColor Red
    Write-Host "   Try these steps:" -ForegroundColor Yellow
    Write-Host "   1. Check if your Node.js server is running on port 8084" -ForegroundColor Gray
    Write-Host "   2. Check Windows Firewall settings" -ForegroundColor Gray
    Write-Host "   3. Verify Docker Desktop is using WSL2 backend" -ForegroundColor Gray
}

Write-Host "`n🔥 Manual test commands:" -ForegroundColor Yellow
Write-Host "Test from container: docker exec $containers wget -q -O- http://host.docker.internal:8084/health" -ForegroundColor Gray
Write-Host "Test from Windows:   curl http://localhost:8084/health" -ForegroundColor Gray

Write-Host "`n✨ Script completed!" -ForegroundColor Green