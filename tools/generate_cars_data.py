import csv
import json
from collections import OrderedDict

INPUT = r"c:\Users\lflen\Downloads\Shay - R3E Spreadsheet - Feuille 1.csv"
OUTPUT = r"c:\Users\lflen\_git\r3e-leaderboard-ui\data\cars.json"

def normalize_cat_wheel(wheel_raw):
    if not wheel_raw: return ''
    s = wheel_raw.lower()
    if 'round' in s and 'flat' in s:
        return 'round (flat)'
    if 'round' in s:
        return 'round'
    if 'gt' in s:
        return 'gt'
    return 'other'

def normalize_cat_trans(traw):
    if not traw: return ''
    s = traw.lower()
    if 'pal' in s: return 'paddles'
    if 'seq' in s: return 'sequential'
    if 'h' in s: return 'h'
    return 'other'

cars_by_class = OrderedDict()
current_class = None

with open(INPUT, newline='', encoding='utf-8') as f:
    reader = csv.DictReader(f)
    for row in reader:
        cls = row.get('CLASS','').strip()
        car = row.get('CAR','').strip()
        if cls:
            current_class = cls
            if current_class not in cars_by_class:
                cars_by_class[current_class] = []
        if not car:
            continue
        # build entry
        entry = {
            'car_class': current_class,
            'car': car,
            'wheel': row.get('WHEEL','').strip(),
            'wheel_cat': normalize_cat_wheel(row.get('WHEEL','').strip()),
            'transmission': row.get('TRANSMISSION','').strip(),
            'transmission_cat': normalize_cat_trans(row.get('TRANSMISSION','').strip()),
            'year': row.get('YEAR','').strip(),
            'power': row.get('POWER','').strip(),
            'weight': row.get('WEIGHT','').strip(),
            'engine': row.get('ENGINE','').strip(),
            'drive': row.get('DRIVE','').strip(),
            'country': row.get('COUNTRY','').strip(),
            'description': row.get('DESCRIPTION','').strip(),
        }
        if current_class is None:
            current_class = 'Unknown'
            cars_by_class.setdefault(current_class, [])
        cars_by_class[current_class].append(entry)

# write list of classes with cars
out = []
for cls, cars in cars_by_class.items():
    out.append({'class': cls, 'cars': cars})

with open(OUTPUT, 'w', encoding='utf-8') as f:
    json.dump(out, f, indent=2, ensure_ascii=False)

print(f'Wrote {len(out)} classes to {OUTPUT}')
