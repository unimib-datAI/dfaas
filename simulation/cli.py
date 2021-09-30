import argparse
import numpy as np

def parse_arguments():
    """
        Method used for parse arguments passed by terminal.
    """
    parser = argparse.ArgumentParser()
    parser.add_argument('-n', '--nodesnum', type=int,
                        help="Represent the number of nodes used in simulation")
    parser.add_argument('-s', '--seed', type=int, default=np.random.randint(4097), required=False,
                        help="Optional param that represent seed used for random number generation")
    parser.add_argument('-p', '--edgeprob', type=float, default=0.1, required=False,
                        help="Optional param that represent probability of creating an edge")

    args = parser.parse_args()

    if args.nodesnum <= 0:
        raise parser.error("Nodes number must be integer, greater than 0")
    if args.seed <= 0:
        raise parser.error("Seed must be an integer, greater than 0")
    if args.edgeprob < 0.0 or args.edgeprob > 1.0:
        raise parser.error("Edge probability must be a float number between 0 and 1")


    return args


def get_args():
    """
        Returns dictionary created with key-value params passed to program.
    """
    kargs = dict(parse_arguments()._get_kwargs())
    return kargs
