Whatsapp Ravi Monitor
---------------------

# Instalando
git clone https://github.com/rodrigomatos87/whatsapp.git<br>
cd whatsapp<br>
make prod


# Se precisasr atualizar o go antes de atualizar a biblioteca
apt-get remove golang -y<br>
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz<br>
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

echo "<br>
export GOROOT=/usr/local/go<br>
export GOPATH=\$HOME/go<br>
export PATH=\$GOPATH/bin:\$GOROOT/bin:\$PATH" >> /root/.profile<br>
source /root/.profile


# Atualizando a biblioteca
apt install golang # Caso ainda não esteja instalado<br>
go get -u go.mau.fi/whatsmeow<br>
make prod<br><br>

mv -f /tmp/ravi-go/ravi-go /var/www/html/go/server<br>
chmod +x /var/www/html/go/server<br><br>

rm -fr /tmp/ravi-go<br><br>

echo '[Unit]<br>
Description=ravi-go<br>
After=network.target<br><br>

[Service]<br>
WorkingDirectory=/var/www/html/go<br>
ExecStart=/var/www/html/go/server<br>
Restart=on-failure<br>
RestartSec=1s<br><br>

[Install]<br>
WantedBy=multi-user.target' > /etc/systemd/system/ravi-go.service<br><br>

# Executando
systemctl daemon-reload<br>
systemctl start ravi-go<br>
systemctl enable ravi-go
