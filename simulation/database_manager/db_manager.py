import sqlite3
from sqlite3 import Error


class DbManager:

    def __init__(self, db_path) -> None:
        self._path = db_path

    def create_connection(self):
        """ Create a database connection to a SQLite database """
        conn = None
        try:
            conn = sqlite3.connect(self._path)
            print("Connection successfully completed to version: {}".format(sqlite3.version))
        except Error as e:
            print(e)
        finally:
            if conn:
                conn.close()
