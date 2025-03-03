# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

"""
This is an example of a simple 3-nodes DFaaS environment.
"""
from mininet.net import Containernet
from mininet.node import Controller
from mininet.cli import CLI
from mininet.log import info, setLogLevel
from dotenv import dotenv_values
import copy


def prepare_env(env, ip):
    _env = copy.deepcopy(env)
    _env['AGENT_LISTEN'] = '/ip4/{}/tcp/6000'.format(ip)
    _env['AGENT_HAPROXY_HOST'] = ip
    return _env


agent_env = dotenv_values('../dfaasagent.env')

setLogLevel('info')

net = Containernet(controller=Controller)
net.addController('c0')

info('*** Adding container\n')
n1 = net.addDocker('n1', ip='10.0.0.1', dcmd='./entrypoint.sh', dimage="dfaas-node:latest", runtime='sysbox-runc',
                   environment=prepare_env(agent_env, '10.0.0.1'))
n2 = net.addDocker('n2', ip='10.0.0.2', dcmd='./entrypoint.sh', dimage="dfaas-node:latest", runtime='sysbox-runc',
                   environment=prepare_env(agent_env, '10.0.0.2'))
n3 = net.addDocker('n3', ip='10.0.0.3', dcmd='./entrypoint.sh', dimage="dfaas-node:latest", runtime='sysbox-runc',
                   environment=prepare_env(agent_env, '10.0.0.3'))

info('*** Setup network\n')
s1 = net.addSwitch('s1')
net.addLink(n1, s1)
net.addLink(n2, s1)
net.addLink(n3, s1)

info('*** Starting network\n')
net.start()

CLI(net)

net.stop()
