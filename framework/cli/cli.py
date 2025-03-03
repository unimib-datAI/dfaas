# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from configuration.config_manager import ConfigManager

config_manager = ConfigManager()

def analyzer_and_simulator_arguments():
    """
        Method used to handle arguments passed by terminal to the simulator
    """
    parser =argparse.ArgumentParser()
    parser.add_argument('-m', '--modeltype', type=str, default="regression", required=False,
                        help="Optional parameter used to choose the model type to use during the simulation (regression, quantile005, quantile095). Default value is \"regression\"")
    
    args = parser.parse_args()
    if args.modeltype != "regression" and args.modeltype != "quantile005" and args.modeltype != "quantile095":
        raise parser.error("Model type can only be \"regression\" \"quantile005\" or \"quantile095\"")
    return args


def simulation_controller_arguments():
    parser = argparse.ArgumentParser()
    parser.add_argument('-n', '--nodesnum', type=int, default=-1, required=False,
                        help="Represent the number of nodes used in simulation")
    parser.add_argument('-s', '--seed', type=int, default=-1, required=False,
                        help="Optional param that represent seed used for random number generation. It cannot be used if expnum is greater than 1")
    parser.add_argument('-p', '--edgeprob', type=float, default=-1.0, required=False,
                        help="Optional param that represent probability of creating an edge")
    parser.add_argument('-o', '--overloaded', nargs='+', type=int, required=False,
                        help="Optional param that represent the percentages of overloaded node to test on the same instance. Default values are 30, 60, 90")
    parser.add_argument('-e', '--expnum', type=int, default=-1, required=False,
                        help="Optional param that represent the number of experiment to perform. If not setted, it will be executed just one experiment")
    parser.add_argument('-m', '--modeltype', type=str, default="regression", required=False,
                        help="Optional parameter used to choose the model type to use during the simulation (regression, quantile005, quantile095). Default value is \"regression\"")
    
    args = parser.parse_args()
    
    if args.modeltype != "regression" and args.modeltype != "quantile005" and args.modeltype != "quantile095":
        raise parser.error("Model type can only be \"regression\" \"quantile005\" or \"quantile095\"")
    if args.nodesnum != -1 and args.nodesnum <= 0:
        raise parser.error("Nodes number must be integer, greater than 0")
    if args.expnum != -1 and args.expnum <= 0:
        raise parser.error("Experiments number must be integer, greater than 0")
    if args.seed != -1 and args.expnum > 1:
        raise parser.error("Seed cannot be setted when experiments number is greater than 1")
    if args.seed != -1 and args.seed <= 0:
        raise parser.error("Seed must be an integer, greater than 0")
    if args.edgeprob != -1.0 and (args.edgeprob < 0.0 or args.edgeprob > 1.0):
        raise parser.error("Edge probability must be a float number between 0 and 1")
    if args.overloaded is not None:
        for rate in args.overloaded:
            if rate < 0 or rate > 100:
                raise parser.error("Overloaded percentage must be integer, grater eq than 0 and lower eq than 100")
    return args

def parse_arguments():
    """
        Method used for parse arguments passed by terminal.
    """
    parser = argparse.ArgumentParser()
    parser.add_argument('-n', '--nodesnum', type=int, default=-1, required=False,
                        help="Represent the number of nodes used in simulation")
    parser.add_argument('-s', '--seed', type=int, default=-1, required=False,
                        help="Optional param that represent seed used for random number generation")
    parser.add_argument('-p', '--edgeprob', type=float, default=-1.0, required=False,
                        help="Optional param that represent probability of creating an edge")
    parser.add_argument('-o', '--overloaded', type=int, default=-1, required=False,
                        help="Optional param that represent the percentages of overloaded node to test on the same instance")

    args = parser.parse_args()

    if args.nodesnum != -1 and args.nodesnum <= 0:
        raise parser.error("Nodes number must be integer, greater than 0")
    if args.seed != -1 and args.seed <= 0:
        raise parser.error("Seed must be an integer, greater than 0")
    if args.edgeprob != -1.0 and (args.edgeprob < 0.0 or args.edgeprob > 1.0):
        raise parser.error("Edge probability must be a float number between 0 and 1")
    if args.overloaded != -1 and (args.overloaded < 0 or args.overloaded > 100):
        raise parser.error("Overloaded percentage must be integer, grater eq than 0 and lower eq than 100")
    return args


def get_args():
    """
        Returns dictionary created with key-value params passed to program.
    """
    kargs = dict(parse_arguments()._get_kwargs())
    return kargs

def get_analyzer_and_simulator_args():
    kargs = dict(analyzer_and_simulator_arguments()._get_kwargs())
    return kargs

def get_simulation_controller_args():
    kargs = dict(simulation_controller_arguments()._get_kwargs())
    return kargs
