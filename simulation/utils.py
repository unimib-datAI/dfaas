import os
import shutil
from datetime import datetime

def create_timestamp_folder(base_path):
    """
    This function create a directory with timestamp as name
    """
    dir_path = base_path + \
        datetime.now().strftime('%Y-%m-%d_%H-%M-%S')
    mydir = os.path.join(
        os.getcwd(),
        dir_path
    )
    try:
        os.makedirs(mydir)
    except OSError as e:
        if e.errno != errno.EEXIST:
            raise  # This was not a "directory exist" error..
    return dir_path


def copy_file(full_file_name, dest):
    """
    Copy file [full_file_name] to [dest]
    """
    shutil.copy(full_file_name, dest)


def copy_dir(src, dest):
    """
    Copy content of directory [src] to directory [dest]
    """
    src_files = os.listdir(src)
    for file_name in src_files:
        full_file_name = os.path.join(src, file_name)
        if os.path.isfile(full_file_name):
            copy_file(full_file_name, dest)


def remove_file(path):
    """
    Remove an existing file
    """
    os.remove(path)


def remove_dir_content(dir):
    """
    Remove all files from a directory [dir]
    """
    files = os.listdir(dir)
    for file_name in files:
        path = os.path.join(dir, file_name)
        if os.path.isfile(path):
            remove_file(path)


def remove_dir_with_content(dir):
    """
    Remove dir [dir] along with all its files
    """
    shutil.rmtree(dir)


def zip_foulder(dir, out_path, format="zip"):
    """
    Zip foulder specified by [dir] in [out_path] using [format] format
    Default format is "zip"
    """
    shutil.make_archive(out_path, format, dir)
