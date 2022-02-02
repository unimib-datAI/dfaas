#!/usr/bin/python
"""
This is an example how to simulate a client server environment.
"""
from mininet.net import Containernet
from mininet.node import Controller
from mininet.cli import CLI
from mininet.link import TCLink
from mininet.log import info, setLogLevel

setLogLevel('info')

net = Containernet(controller=Controller)
net.addController('c0')

info('*** Adding server and client container\n')
platform = net.addHost('platform', ip='10.0.0.251')
proxy = net.addDocker('proxy', ip='10.0.0.252', dcmd="haproxy -f /usr/local/etc/haproxy/haproxy.cfg",
                      dimage="proxy:latest")
agent = net.addDocker('agent', ip='10.0.0.253', dimage="agent:latest")

info('*** Setup network\n')
s1 = net.addSwitch('s1')
s2 = net.addSwitch('s2')
net.addLink(platform, s1)
net.addLink(s1, s2, cls=TCLink, delay='100ms', bw=1)
net.addLink(s2, proxy)
net.addLink(proxy, s2)
net.addLink(proxy, agent)
net.start()

info('*** Starting to execute commands\n')

info('Execute: client.cmd("time curl 10.0.0.251")\n')
info(proxy.cmd("time curl 10.0.0.251") + "\n")

info('Execute: client.cmd("time curl 10.0.0.251/hello/42")\n')
info(agent.cmd("time curl 10.0.0.252") + "\n")

CLI(net)

net.stop()
