import datetime
from database_manager.exp_db_manager import ExpDbManager
from data_loader.config_request import ConfigRequest
from data_loader.function_request import FunctionRequest

db_manager = ExpDbManager("database_design/experiment_db")
# db_manager.create_connection()
# db_manager.create_tables()

# db_manager.insert_node("node_1", 2, 1)
# db_manager.insert_function("funca", "Very simple function")
# db_manager.insert_function("qrcode", "Qrcode generation function")
# db_manager.insert_exp_instant(datetime.datetime.now(), 1)
# db_manager.insert_metric("ram_usage", "node", "%", 34, "RAM usage for a node", 1, node_id = 1)
# db_manager.insert_metric("afet", "func", "sec", 0.000000005, "Average Function Execution time", 1, function_id = 1)
# db_manager.insert_deploy(1, 1, 150, 5, 50)
# db_manager.insert_deploy(1, 2, 20, 2, 10)

db_manager.select_example()

db_manager.get_metrics(
    ConfigRequest(
        "node_1",
        [
            FunctionRequest("funca", 5, 50),
            FunctionRequest("qrcode", 2, 10)
        ]
    )
)
