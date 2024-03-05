import subprocess
import sys
import re

# This script searches for PIDs of functions running in Docker containers based on their names.
def main():
    functions = []
    containers_id = {}
    # Get functions names from terminal
    for function_name in sys.argv[1:]:
        functions.append(function_name)
        containers_id[function_name] = []
    print("Searching PID for these functions: ", functions)

    # Get all the containers running
    temp = subprocess.Popen(["docker", "container", "ls"], stdout=subprocess.PIPE)
    rows = str(temp.communicate())
    rows = rows.split("\\n")
    for row in rows:
        data = row.split()
        for function_name in functions:
            if (data[1].__contains__(function_name) or (data[1].__contains__("ghcr.io/openfaas/") and str(data[-1:]).__contains__(function_name))):
                containers_id[function_name].append(data[0])
    
    functions_pids = {}
    for function_name in functions:
        if (len(containers_id[function_name]) > 0):
            functions_pids[function_name] = []
            for container in containers_id[function_name]:
                temp = subprocess.Popen(['docker', 'inspect', '-f', '\'{{.State.Pid}}\'', container], stdout = subprocess.PIPE)
                functions_pids[function_name].append(''.join(re.findall('\d+', str(temp.communicate()))))
        else:
            raise Exception("Error! Container ID not found for function ", function_name)
    print("List of PIDs for the requested functions: ", functions_pids)  
  
if __name__ == '__main__':
    main()
