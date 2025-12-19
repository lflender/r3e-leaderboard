<?php
// Simple PHP reverse proxy to forward /api/* to the backend
// Place this file at /public/api/index.php on your server.

// Try multiple backends in order to work on shared hosting where 127.0.0.1 is isolated
$candidates = [];
$candidates[] = 'http://127.0.0.1:8080';
if (!empty($_SERVER['SERVER_ADDR'])) {
    $candidates[] = 'http://' . $_SERVER['SERVER_ADDR'] . ':8080';
}
// Try resolving the host itself (public IP)
if (!empty($_SERVER['HTTP_HOST'])) {
    $candidates[] = 'http://' . $_SERVER['HTTP_HOST'] . ':8080';
    $resolved = gethostbyname($_SERVER['HTTP_HOST']);
    if (filter_var($resolved, FILTER_VALIDATE_IP)) {
        $candidates[] = 'http://' . $resolved . ':8080';
    }
}
// Optional override via env/config: set BACKEND_BASE in hosting panel or .htaccess (SetEnv)
$override = getenv('BACKEND_BASE');
if (!$override && isset($_SERVER['BACKEND_BASE'])) { $override = $_SERVER['BACKEND_BASE']; }
if (!empty($override)) { array_unshift($candidates, rtrim($override, '/')); }

// Optional: a Unix socket for local proxying without TCP (requires libcurl with unix sockets)
$unixSock = getenv('BACKEND_SOCKET');
if (!$unixSock && isset($_SERVER['BACKEND_SOCKET'])) { $unixSock = $_SERVER['BACKEND_SOCKET']; }

// Preserve the full API path and query string
$uri = $_SERVER['REQUEST_URI']; // e.g., /api/status?x=1

$lastError = null;
$response = null;
$statusCode = 502;
$rawHeaders = '';
$body = '';

// If a unix socket is configured, try it first
if (!empty($unixSock)) {
    $ch = curl_init('http://localhost' . $uri);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_HEADER, true);
    curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $_SERVER['REQUEST_METHOD']);
    if (defined('CURLOPT_UNIX_SOCKET_PATH')) {
        curl_setopt($ch, CURLOPT_UNIX_SOCKET_PATH, $unixSock);
    }
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);
    $headers = [];
    if (function_exists('getallheaders')) {
        foreach (getallheaders() as $name => $value) {
            if (strtolower($name) === 'host') continue;
            $headers[] = $name . ': ' . $value;
        }
    }
    $headers[] = 'X-Forwarded-Proto: ' . (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off' ? 'https' : 'http');
    if (!empty($_SERVER['HTTP_HOST'])) { $headers[] = 'X-Forwarded-Host: ' . $_SERVER['HTTP_HOST']; }
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    $method = strtoupper($_SERVER['REQUEST_METHOD']);
    if ($method !== 'GET' && $method !== 'HEAD') {
        $reqBody = file_get_contents('php://input');
        curl_setopt($ch, CURLOPT_POSTFIELDS, $reqBody);
    }
    $resp = curl_exec($ch);
    if ($resp !== false) {
        $headerSize = curl_getinfo($ch, CURLINFO_HEADER_SIZE);
        $statusCode = curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        $rawHeaders = substr($resp, 0, $headerSize);
        $body = substr($resp, $headerSize);
        curl_close($ch);
        http_response_code($statusCode);
        $skip = ['transfer-encoding','content-length','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','upgrade'];
        $lines = preg_split("/(\r\n|\n|\r)/", trim($rawHeaders));
        foreach ($lines as $line) {
            if (strpos($line, ':') !== false) {
                [$name, $value] = explode(':', $line, 2);
                if (in_array(strtolower(trim($name)), $skip, true)) continue;
                header(trim($name) . ':' . trim($value), true);
            }
        }
        echo $body;
        exit;
    }
    $lastError = curl_error($ch);
    curl_close($ch);
}

foreach ($candidates as $backendBase) {
    $targetUrl = $backendBase . $uri;
    $ch = curl_init($targetUrl);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_HEADER, true);
    curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $_SERVER['REQUEST_METHOD']);
    // Quick connect timeout to allow fast fallback
    if (defined('CURLOPT_CONNECTTIMEOUT_MS')) {
        curl_setopt($ch, CURLOPT_CONNECTTIMEOUT_MS, 500);
    } else {
        curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 1);
    }
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);

    // Forward headers except Host
    $headers = [];
    if (function_exists('getallheaders')) {
        foreach (getallheaders() as $name => $value) {
            if (strtolower($name) === 'host') continue;
            $headers[] = $name . ': ' . $value;
        }
    }
    // Inform backend of original protocol/host
    $headers[] = 'X-Forwarded-Proto: ' . (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off' ? 'https' : 'http');
    if (!empty($_SERVER['HTTP_HOST'])) {
        $headers[] = 'X-Forwarded-Host: ' . $_SERVER['HTTP_HOST'];
    }
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);

    // Forward body for non-GET/HEAD requests
    $method = strtoupper($_SERVER['REQUEST_METHOD']);
    if ($method !== 'GET' && $method !== 'HEAD') {
        $reqBody = file_get_contents('php://input');
        curl_setopt($ch, CURLOPT_POSTFIELDS, $reqBody);
    }

    curl_setopt($ch, CURLOPT_FOLLOWLOCATION, false);

    $resp = curl_exec($ch);
    if ($resp === false) {
        $lastError = curl_error($ch);
        curl_close($ch);
        // Try next candidate
        continue;
    }

    $headerSize = curl_getinfo($ch, CURLINFO_HEADER_SIZE);
    $statusCode = curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
    $rawHeaders = substr($resp, 0, $headerSize);
    $body = substr($resp, $headerSize);
    curl_close($ch);
    // Success if we got any HTTP response
    $response = $resp;
    break;
}

if ($response === null) {
    http_response_code(502);
    header('Content-Type: application/json');
    $detail = $lastError ?: 'No backend reachable';
    echo json_encode(['error' => 'Bad Gateway', 'detail' => $detail, 'tried' => $candidates], JSON_UNESCAPED_SLASHES);
    exit;
}

http_response_code($statusCode);

// Relay headers, skipping hop-by-hop headers to avoid conflicts
$skip = ['transfer-encoding','content-length','connection','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','upgrade'];
$lines = preg_split("/(\r\n|\n|\r)/", trim($rawHeaders));
foreach ($lines as $line) {
    if (strpos($line, ':') !== false) {
        [$name, $value] = explode(':', $line, 2);
        if (in_array(strtolower(trim($name)), $skip, true)) continue;
        header(trim($name) . ':' . trim($value), true);
    }
}

echo $body;
