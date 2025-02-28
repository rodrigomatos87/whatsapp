Whatsapp Ravi Monitor
---------------------

# Instalando
git clone https://github.com/rodrigomatos87/whatsapp.git
cd whatsapp
make prod


# Se precisasr atualizar o go antes de atualizar a biblioteca
apt-get remove golang -y<br>
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

echo "
export GOROOT=/usr/local/go
export GOPATH=\$HOME/go
export PATH=\$GOPATH/bin:\$GOROOT/bin:\$PATH" >> /root/.profile
source /root/.profile


# Atualizando a biblioteca
apt install golang # Caso ainda não esteja instalado
go get -u go.mau.fi/whatsmeow
make prod

mv -f /tmp/ravi-go/ravi-go /var/www/html/go/server
chmod +x /var/www/html/go/server

rm -fr /tmp/ravi-go

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
