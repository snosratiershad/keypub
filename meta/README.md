# setup a debian machine

# nginx + certbot:
3. create and set permissions for index folder:
```
sudo mkdir -p FOLDER
sudo chown -R www-data:www-data FOLDER 
sudo chmod -R 755 FOLDER
```
Nginx config:
```
sudo vim /etc/nginx/sites-available/default
```
Nginx config:
```# shellbox.dev configuration
server {
    listen 80;
    listen [::]:80;
    server_name shellbox.dev www.shellbox.dev;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name shellbox.dev www.shellbox.dev;

    ssl_certificate /etc/letsencrypt/live/shellbox.dev/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/shellbox.dev/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root /home/ubuntu/prod/shellbox_static_web;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}

# keypub.sh configuration
server {
    listen 80;
    listen [::]:80;
    server_name keypub.sh www.keypub.sh;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name keypub.sh www.keypub.sh;

    ssl_certificate /etc/letsencrypt/live/keypub.sh/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/keypub.sh/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root /home/ubuntu/prod/keypub_static_web;  # Changed this to a different directory
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
```
Test and reload Nginx
```
sudo nginx -t
sudo nginx -s reload
```
3. Setup SSL with Certbot, for each domain:
```
sudo certbot --nginx -d yourdomain.com -d www.yourdomain.com
```
test and reload again.

# hompage

port fw: `ssh -p 2222 -NfL 5173:localhost:5173 hdev`

# sshd config (/etc/ssh/sshd_config):
```
Include /etc/ssh/sshd_config.d/*.conf
Port 2222

PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM no

KbdInteractiveAuthentication no
X11Forwarding no
PrintMotd no
AcceptEnv LANG LC_*
Subsystem       sftp    /usr/lib/openssh/sftp-server
PermitRootLogin no
```
test shd:
`sudo sshd -T`

restart sshd:
`sudo systemctl restart sshd`.

# stuff

mail service: resend

analytics service: plausible

count min sketch (not currently used): https://github.com/shenwei356/countminsketch

exponential ratelimiting: https://dotat.at/@/2024-09-02-ewma.html

using go jet: https://github.com/go-jet/jet


