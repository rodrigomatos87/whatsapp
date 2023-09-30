# whatsapp
Whatsapp Ravi Monitor
---------------------

# Instalando
git clone https://github.com/rodrigomatos87/whatsapp.git
cd whatsapp
make prod

mv -f /tmp/ravi-go/ravi-go /var/www/html/go/server
chmod +x /var/www/html/go/server

echo '[Unit]
Description=ravi-go
After=network.target

[Service]
WorkingDirectory=/var/www/html/go
ExecStart=/var/www/html/go/server
Restart=on-failure
RestartSec=1s

[Install]
WantedBy=multi-user.target' > /etc/systemd/system/ravi-go.service

# Executando
systemctl daemon-reload
systemctl start ravi-go
systemctl enable ravi-go
