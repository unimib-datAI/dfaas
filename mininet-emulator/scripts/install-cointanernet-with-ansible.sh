sudo apt-get install ansible
git clone https://github.com/containernet/containernet.git
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml