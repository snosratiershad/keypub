#!/bin/bash

# Exit on any error
set -e

# 1. Create ubuntu user with sudo privileges if it doesn't exist
if ! id "ubuntu" &>/dev/null; then
    useradd -m -s /bin/bash ubuntu
    usermod -aG sudo ubuntu
else
    echo "User 'ubuntu' already exists, skipping user creation"
fi

apt-get update
apt-get upgrade

# 2 & 3. Configure SSH to disable password authentication and enable only certificate-based login
cat >> /etc/ssh/sshd_config << 'EOF'
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM no
PermitRootLogin no
EOF

# Restart SSH service (handling both possible service names)
if systemctl list-units --full -all | grep -Fq "ssh.service"; then
    systemctl restart ssh
elif systemctl list-units --full -all | grep -Fq "sshd.service"; then
    systemctl restart sshd
else
    echo "Warning: Could not find SSH service to restart"
fi

# 4. Copy SSH certificate for ubuntu user
mkdir -p /home/ubuntu/.ssh
cp -r /root/.ssh/authorized_keys /home/ubuntu/.ssh/
chown -R ubuntu:ubuntu /home/ubuntu/.ssh
chmod 700 /home/ubuntu/.ssh
chmod 600 /home/ubuntu/.ssh/authorized_keys

# 5. Set ubuntu user password
echo "ubuntu:ubuntu_321^" | chpasswd

# 6. Install zsh and git
apt-get install -y zsh git libssl-dev pkg-config gcc build-essential sqlite3 certbot python3-certbot-nginx nginx

# Change ubuntu's shell to zsh (doing this as root)
chsh -s $(which zsh) ubuntu

# 7 & 8. Switch to ubuntu user and install devbox
su - ubuntu << 'USERSCRIPT'
# Install devbox
curl -fsSL https://get.jetify.com/devbox | bash

# 9. Install global tools with devbox
devbox global add zip unzip uv ripgrep fzf nodejs neovim zellij rustup go gopls
devbox global install
eval "$(devbox global shellenv)"
refresh-global
uv venv --python=3.13
source .venv/bin/activate
uv pip install ipython pydantic sqlite-web

# 10. Clone neovim config
mkdir -p ~/.config
git clone https://github.com/skariel/kickstart.nvim ~/.config/nvim

# 11. Install oh-my-zsh and configure devbox initialization
# First, save the devbox initialization command
echo 'eval "$(devbox global shellenv)"' > ~/.zshrc_temp
echo 'refresh-global' >> ~/.zshrc_temp
echo "source /home/ubuntu/.venv/bin/activate" >> .zshrc_temp
echo "export PATH=$PATH:/home/ubuntu/.cargo/bin:/home/ubuntu/go/bin" >> .zshrc_temp

# Install oh-my-zsh (which will create a new .zshrc)
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended

# Append devbox initialization to the new .zshrc
cat ~/.zshrc_temp >> ~/.zshrc
rm ~/.zshrc_temp
USERSCRIPT

echo "Setup completed successfully!"
