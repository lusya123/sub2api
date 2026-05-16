# XueDingToken Codex Windows Installer Static Deploy

Serve these files from the existing main domain static layer:

```text
https://xuedingtoken.com
```

Required static paths:

```text
/install-codex-win-bootstrap.ps1
/install-codex-win.ps1
/downloads/codex/XueDingToken-Codex-Installer-Windows-x64.exe
/downloads/codex/XueDingToken-Codex-Installer-Windows-x64.exe.sha256
```

The frontend-generated command points to:

```text
https://xuedingtoken.com/install-codex-win-bootstrap.ps1
```

The bootstrap downloads the EXE from `https://xuedingtoken.com/downloads/codex/`.

Example Nginx server:

```nginx
location = /install-codex-win-bootstrap.ps1 {
  root /var/www/xdt-installer;
  default_type text/plain;
  add_header Cache-Control "public, max-age=300";
  try_files $uri =404;
}

location = /install-codex-win.ps1 {
  root /var/www/xdt-installer;
  default_type text/plain;
  add_header Cache-Control "public, max-age=300";
  try_files $uri =404;
}

location ^~ /downloads/codex/ {
  root /var/www/xdt-installer;
  add_header Cache-Control "public, max-age=3600";
  try_files $uri =404;
}
```

Build the Windows EXE after changing the payload:

```bash
python3 tools/xdt-codex-installer-win/build.py
```

Then sync the static files:

```bash
install -D -m 0644 frontend/public/install-codex-win-bootstrap.ps1 /var/www/xdt-installer/install-codex-win-bootstrap.ps1
install -D -m 0644 frontend/public/install-codex-win.ps1 /var/www/xdt-installer/install-codex-win.ps1
rsync -av frontend/public/downloads/codex/ /var/www/xdt-installer/downloads/codex/
```
