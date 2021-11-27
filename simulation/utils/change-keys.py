import os
import sys
import json

def dict_key_substitution(data, old, new):
    """
    Utility function used to substitute dictionary key
    """
    data[new] = data[old]
    del data[old]


path = sys.argv[1]
print(path)

json_files = [pos_json for pos_json in os.listdir(path) if pos_json.endswith('.json')]

print(json_files)

for exp in json_files:
    exp = os.path.join(path, exp)
    f = open(exp)
    json_doc = json.load(f)  # Return json file as a dictionary

    dict_key_substitution(json_doc["input"], "funcb_num", "qrcode_num")
    dict_key_substitution(json_doc["input"], "funcc_num", "ocr_num")
    dict_key_substitution(json_doc["input"], "funcb_wl", "qrcode_wl")
    dict_key_substitution(json_doc["input"], "funcc_wl", "ocr_wl")
    
    with open(exp, 'w', encoding='utf-8') as f:
        json.dump(json_doc, f, ensure_ascii=False, indent=4)
