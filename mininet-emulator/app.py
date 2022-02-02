#!/usr/bin/python
"""
This is an example how to simulate a client server environment.
"""
from mininet.net import Containernet
from mininet.node import Controller
from mininet.cli import CLI
from mininet.log import info, setLogLevel

setLogLevel('info')

net = Containernet(controller=Controller)
net.addController('c0')

info('*** Adding platform, proxy and agent\n')
platform = net.addDocker('platform', ip='10.0.0.251', dimage="platform:latest")
proxy = net.addDocker('proxy', ip='10.0.0.252', dimage="proxy:latest")
agent = net.addDocker('agent', ip='10.0.0.253', dimage="agent:latest")

info('*** Setup network\n')

s1 = net.addSwitch('s1')
s2 = net.addSwitch('s2')
net.addLink(platform, s1)
net.addLink(s1, s2)
net.addLink(s2, proxy)
net.addLink(s2, agent)
net.start()

info('*** Starting to execute commands\n')

info('Execute: proxy.cmd("ping 10.0.0.251")\n')
info(proxy.cmd("ping 10.0.0.251") + "\n")

info('Execute: agent.cmd("ping 10.0.0.252")\n')
info(agent.cmd("ping 10.0.0.252") + "\n")

CLI(net)

net.stop()
