@echo off
REM NullGate WAF — Live Attack Demonstration (Windows)
REM Fires real attack payloads at the WAF, shows live blocking + metrics updates.
REM Perfect for graduation defense presentations.

setlocal enabledelayedexpansion

if "%WAF_URL%"=="" set WAF_URL=http://127.0.0.1:8080
if "%ADMIN_URL%"=="" set ADMIN_URL=http://127.0.0.1:9091
if "%ATTACK_DELAY%"=="" set ATTACK_DELAY=2

echo.
echo ===============================================================
echo                  NullGate WAF — Live Demo
echo          Real-time Attack Detection ^& Blocking
echo ===============================================================
echo.
echo Setup:
echo   WAF:          %WAF_URL%
echo   Admin Panel:  %ADMIN_URL%
echo   Attack delay: %ATTACK_DELAY%s
echo.
echo Instructions:
echo   1. Open the admin dashboard: %ADMIN_URL%
echo   2. Watch the KPIs, traffic chart, and attack categories update live
echo   3. Observe security events appearing in real-time
echo.
pause

echo.
echo Starting attack sequence...
echo.

set TOTAL=17
set BLOCKED=0
set ALLOWED=0
set NUM=0

call :attack "sqli" "' OR '1'='1' --" "SQL Injection (authentication bypass)"
call :attack "sqli" "1 UNION SELECT username,password FROM users" "SQL Injection (data exfiltration)"
call :attack "xss" "<script>document.cookie</script>" "Cross-Site Scripting (cookie theft)"
call :attack "xss" "<img src=x onerror=alert(1)>" "XSS (DOM-based)"
call :attack "rce" ";cat /etc/passwd" "Remote Code Execution (command injection)"
call :attack "rce" "$(whoami)" "RCE (shell substitution)"
call :attack "lfi" "../../../../etc/passwd" "Local File Inclusion"
call :attack "lfi" "php://filter/convert.base64-encode/resource=index" "LFI (PHP wrapper)"
call :attack "ssrf" "http://169.254.169.254/latest/meta-data/" "Server-Side Request Forgery (AWS metadata)"
call :attack "ssrf" "http://127.0.0.1:8080/admin" "SSRF (internal service scan)"
call :attack "xxe" "<!DOCTYPE foo [<!ENTITY x SYSTEM \"file:///etc/passwd\">]>" "XML External Entity"
call :attack "ssti" "{{7*7}}" "Server-Side Template Injection"
call :attack "nosqli" "{\"user\":{\"$ne\":null}}" "NoSQL Injection (MongoDB)"
call :attack "deserialize" "rO0ABXNy" "Java Deserialization"
call :attack "proto_pollution" "{\"__proto__\":{\"admin\":true}}" "Prototype Pollution"
call :attack "crlf" "%%0d%%0aSet-Cookie: injected=1" "CRLF Injection (header manipulation)"
call :attack "scanner" "sqlmap/1.7" "Scanner Detection (automated tool)"

echo.
echo ================================================================
echo Demo Complete!
echo.
echo Summary:
echo   Blocked:  %BLOCKED% / %TOTAL%
echo   Allowed:  %ALLOWED% / %TOTAL%

set /a BLOCK_RATE=(%BLOCKED%*100)/%TOTAL%
echo   Detection Rate: %BLOCK_RATE%%%
echo.

if %BLOCKED% EQU %TOTAL% (
    echo PERFECT SCORE — All attacks blocked!
) else if %BLOCKED% GEQ 15 (
    echo GOOD — Most attacks blocked, tune paranoia level for 100%%
) else (
    echo FAILED — Significant attacks passed through
)

echo.
echo Check the admin dashboard to see:
echo   * Live traffic chart showing the attack burst
echo   * Attack categories breakdown (SQLi, XSS, RCE, ...)
echo   * Security events log with blocked payloads
echo   * Updated block rate and metrics
echo.
pause
exit /b

:attack
set /a NUM+=1
set "cat=%~1"
set "payload=%~2"
set "desc=%~3"

echo Attack %NUM%/%TOTAL% — %desc%
echo   Category: %cat%
echo   Payload:  %payload%

REM Fire the attack with curl
for /f %%i in ('curl -s -o nul -w "%%{http_code}" -H "User-Agent: Mozilla/5.0" -m 5 "%WAF_URL%/?q=%payload%" 2^>nul') do set HTTP_CODE=%%i

if "%HTTP_CODE%"=="403" (
    echo   Result:   BLOCKED (HTTP %HTTP_CODE%^)
    set /a BLOCKED+=1
) else if "%HTTP_CODE%"=="406" (
    echo   Result:   BLOCKED (HTTP %HTTP_CODE%^)
    set /a BLOCKED+=1
) else if "%HTTP_CODE%"=="400" (
    echo   Result:   BLOCKED (HTTP %HTTP_CODE%^)
    set /a BLOCKED+=1
) else if "%HTTP_CODE%"=="200" (
    echo   Result:   ALLOWED (HTTP %HTTP_CODE%^) [VULNERABILITY]
    set /a ALLOWED+=1
) else if "%HTTP_CODE%"=="301" (
    echo   Result:   ALLOWED (HTTP %HTTP_CODE%^) [VULNERABILITY]
    set /a ALLOWED+=1
) else if "%HTTP_CODE%"=="302" (
    echo   Result:   ALLOWED (HTTP %HTTP_CODE%^) [VULNERABILITY]
    set /a ALLOWED+=1
) else (
    echo   Result:   UNKNOWN (HTTP %HTTP_CODE%^)
)

echo.
if %NUM% LSS %TOTAL% timeout /t %ATTACK_DELAY% /nobreak >nul
exit /b
