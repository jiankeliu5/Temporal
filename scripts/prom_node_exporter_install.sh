#! /bin/bash

# installs prometheus node exporter

cd ~
wget https://github.com/prometheus/node_exporter/releases/download/v0.16.0/node_exporter-0.16.0.linux-amd64.tar.gz
tar zxvf node_exporter*.tar.gz
rm *.gz
mkdir /usr/local/bin/prom_node_exporter
mv ~/node_exporter*/* /usr/local/bin/prom_node_exporter
cp ~/go/src/github.com/RTradeLtd/Temporal/scripts/prom_exporter_start.sh /boot_scripts/prom_exporter_start.sh
chmod a+x /boot_scripts/prom_exporter_start.sh
cp ~/go/src/github.com/RTradeLtd/Temporal/configs/prom_node_exporter.service /etc/systemd/system
systemctl enable prom_node_exporter.service