import sqlite3
from sqlite3 import Error

class DbManager():
    def __init__(self, db_path) -> None:
        self._path = db_path
        self._conn = None

    def create_connection(self):
        """ Create a database connection to a SQLite database """
        try:
            self._conn = sqlite3.connect(self._path)
            print(sqlite3.version)
        except Error as e:
            print(e)
        finally:
            if self._conn:
                self._conn.close()

