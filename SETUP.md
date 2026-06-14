# VM Hosting Setup

This is a quick note-to-self on setting up a fresh Debian (or its derivatives) VM for hosting a bot.

## Basics & SSH
```sh
sudo apt update
sudo apt upgrade
```

## SSH
`sudo vim /etc/ssh/sshd_config`

```
Port 22
Port somethingelse
```

```sh
sudo sshd -t  # Test config
sudo systemctl restart sshd
```

## Service User
```sh
sudo useradd -m snoozybot
sudo -u snoozybot mkdir /home/snoozybot/.ssh
sudo -u snoozybot vim /home/snoozybot/.ssh/authorized_keys
sudo visudo -f /etc/sudoers.d/snoozybot
```

```
snoozybot ALL=(ALL:ALL) NOPASSWD: /usr/bin/systemctl restart snoozybot.service
```

## Postgres
```sh
sudo apt install posgresql
sudo -u postgres psql -c "CREATE USER snoozybot LOGIN PASSWORD 'xxxxxx';"
sudo -u postgres psql -c "CREATE DATABASE snoozybot_v3 OWNER snoozybot;"
# Run db restore etc here...
```

```SQL
CREATE USER snoozybot WITH PASSWORD '...';
```

## DDNS
```sh
sudo vim /etc/systemd/system/ddns.service
sudo systemctl daemon-reload
sudo systemctl enable ddns.service
```

```ini
[Unit]
Description=Update DDNS
After=network.target

[Service]
Type=exec
WorkingDirectory=/home/snoozybot
ExecStart=/usr/bin/curl -s ...
User=snoozybot
Group=snoozybot
RemainAfterExit=true

[Install]
WantedBy=default.target
```

## Create dotenv
```sh
sudo -u snoozybot vim /home/snoozybot/.env
```

## Create systemd service
```sh
sudo vim /etc/systemd/system/snoozybot.service
sudo systemctl daemon-reload
sudo systemctl enable snoozybot.service
```

```ini
[Unit]
Description=Snoozybot
After=network.target

[Service]
Type=exec
WorkingDirectory=/home/snoozybot
ExecStart=/home/snoozybot/snoozybot
User=snoozybot
Group=snoozybot
Restart=always
RestartSec=15
KillSignal=SIGTERM

[Install]
WantedBy=default.target
```