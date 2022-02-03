#!/usr/bin/python3

from mininet.net import Containernet
from mininet.node import Controller
from mininet.cli import CLI
from mininet.log import info, setLogLevel


setLogLevel('info')

net = Containernet(controller=Controller)

info('*** Adding hosts, switch and controller\n')
platform = net.addHost('platform')
proxy = net.addDocker('proxy', dimage="proxy:latest")
agent = net.addDocker('agent', dimage="agent:latest")
switch = net.addSwitch('s1')
controller = net.addController('c0')

info('*** Setup network\n')
net.addLink(platform, switch)
net.addLink(proxy, switch)
net.addLink(agent, switch)
net.addNAT().configDefault()
net.start()

info('*** Testing connectivity\n')
net.ping([platform, proxy, agent])

CLI(net)

net.stop()
