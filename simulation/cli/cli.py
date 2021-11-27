import argparse

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
    parser.add_argument('-i', '--instance', type=str, default="", required=False,
                        help="Optional param that represent path of an existing json instance file")

    args = parser.parse_args()

    if args.nodesnum != -1 and args.nodesnum <= 0:
        raise parser.error("Nodes number must be integer, greater than 0")
    if args.seed != -1 and args.seed <= 0:
        raise parser.error("Seed must be an integer, greater than 0")
    if args.edgeprob != -1.0 and (args.edgeprob < 0.0 or args.edgeprob > 1.0):
        raise parser.error("Edge probability must be a float number between 0 and 1")
    if args.instance != "" and (args.nodesnum != -1 or args.seed != -1 or args.edgeprob != -1.0):
        raise parser.error("Instance flag can't be combined with others flags")

    return args


def get_args():
    """
        Returns dictionary created with key-value params passed to program.
    """
    kargs = dict(parse_arguments()._get_kwargs())
    return kargs
