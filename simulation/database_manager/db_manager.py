import sqlite3
from sqlite3 import Error


class DbManager:
    """
    Class that represent database connection manager
    """

    def __init__(self, db_path) -> None:
        self.__path = db_path

    def create_connection(self):
        """ Create a database connection to a SQLite database """
        conn = None
        try:
            conn = sqlite3.connect(self.__path)
            print("Connection successfully completed to version: {}".format(sqlite3.version))
        except Error as e:
            print(e)
        finally:
            if conn:
                conn.close()
