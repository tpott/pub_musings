# update_plaid.py
# Trevor Pottinger
# Thu Dec 28 14:25:44 PST 2023

import argparse
import csv
from decimal import Decimal
from typing import (Dict, List, NewType, Optional, Tuple, TypedDict)

import requests


Filename = NewType('Filename', str)

class Login(TypedDict):
	itemID: str
	accessToken: str

class Account(TypedDict):
	mask: str
	accountID: str
	itemID: str

class Balance(TypedDict):
	mask: str
	accountID: str
	availableBalance: Decimal
	currentBalance: Decimal
	limit: Decimal

class Secrets(TypedDict):
	development: Optional[str]
	production: Optional[str]


DEFAULT_LOGINS = 'private/logins.csv'
DEFAULT_ACCOUNTS = 'private/accounts.csv'
DEFAULT_CLIENT_ID = 'private/client_id.txt'
DEFAULT_CLIENT_DEV_SECRET = 'private/client_dev_secret.txt'
DEFAULT_CLIENT_PROD_SECRET = 'private/client_prod_secret.txt'


def readLogins(logins: Filename) -> List[Login]:
	data: List[Login] = []
	with open(logins, 'r') as loginsFile:
		csvreader = csv.DictReader(loginsFile)
		if csvreader.fieldnames is None:
			raise ValueError(f'The logins file had no header row: {logins}')
		item_id_field = 'item_id'
		access_token_field = 'access_token'
		for field in csvreader.fieldnames:
			lower = field.lower()
			if 'item' in lower and 'id' in lower:
				item_id_field = field
			if 'access' in lower and 'token' in lower:
				access_token_field = field
		for row in csvreader:
			if row[item_id_field] == '' or row[access_token_field] == '':
				continue
			data.append({
				'itemID': row[item_id_field],
				'accessToken': row[access_token_field],
			})
	return data


def readAccounts(accounts: Filename) -> List[Account]:
	data: List[Account] = []
	with open(accounts, 'r') as accountsFile:
		csvreader = csv.DictReader(accountsFile)
		if csvreader.fieldnames is None:
			raise ValueError(f'The accounts file had no header row: {accounts}')
		mask_field = 'mask'
		account_id_field = 'account_id'
		item_id_field = 'item_id'
		for field in csvreader.fieldnames:
			lower = field.lower()
			if 'mask' in lower:
				mask_field = field
			if 'account' in lower and 'id' in lower:
				account_id_field = field
			if 'item' in lower and 'id' in lower:
				item_id_field = field
		for row in csvreader:
			if row[mask_field] == '' or row[account_id_field] == '' or row[item_id_field] == '':
				continue
			# TODO add optional limit for credit accounts
			data.append({
				'mask': row[mask_field],
				'accountID': row[account_id_field],
				'itemID': row[item_id_field],
			})
	return data


def fetchAccounts(
	client_id: str,
	secrets: Secrets,
	logins: List[Login],
) -> Tuple[List[Account], List[Balance]]:
	accounts: List[Account] = []
	balances: List[Balance] = []
	for login in logins:
		environment = 'development'
		if 'production' in login['accessToken']:
			environment = 'production'
		secret = secrets[environment]
		url = f'https://{environment}.plaid.com/accounts/get'
		print(login['itemID'])
		if secret is None:
			raise ValueError(f'No configured client secret for environment: {environment}')
		print(f"curl -X POST {url} -H 'Content-Type: application/json' -d '{{\"client_id\": \"'$CLIENT_ID'\", \"secret\": \"'$SECRET'\", \"access_token\": \"{login['accessToken']}\"}}'")
		response = requests.post(url, headers={'Content-Type': 'application/json'}, json={
			'client_id': client_id,
			'secret': secret,
			'access_token': login['accessToken'],
		})
		print(response)
		if response.status_code != 200:
			print(f'Non-200 response code: {response.status_code}')
			print(response.text)
			continue
		# TODO add optional limit for credit accounts
		json_data = response.json()
		for acc in json_data['accounts']:
			accounts.append({
				'mask': acc['mask'], 
				'accountID': acc['account_id'],
				'itemID': json_data['item']['item_id'],
			})
			balances.append({
				'mask': acc['mask'], 
				'accountID': acc['account_id'],
				'availableBalance': acc['balances']['available'],
				'currentBalance': acc['balances']['current'],
				'limit': acc['balances']['limit'],
			})
	return (accounts, sorted(balances, key=lambda x: x['mask']))


def main() -> None:
	parser = argparse.ArgumentParser('Update some data using plaid access tokens')
	parser.add_argument('--logins', help='The path to the item IDs and access tokens')
	parser.add_argument('--accounts', help='The path to the item IDs and account IDs')
	parser.add_argument('--client_id', help='The path to the client ID')
	parser.add_argument('--dev_secret', help='The path to the client development secret')
	parser.add_argument('--prod_secret', help='The path to the client production secret')
	args = parser.parse_args()
	print(args)

	logins_file = args.logins
	if logins_file is None:
		logins_file = Filename(DEFAULT_LOGINS)

	print(logins_file)
	logins = readLogins(logins_file)
	print(logins)

	accounts_file = args.accounts
	if accounts_file is None:
		accounts_file = Filename(DEFAULT_ACCOUNTS)

	print(accounts_file)
	accounts = readAccounts(accounts_file)
	print(accounts)
	account_ids = set(map(lambda x: x['accountID'], accounts))

	client_id_file = args.client_id
	if client_id_file is None:
		client_id_file = Filename(DEFAULT_CLIENT_ID)
	client_id = open(client_id_file, 'r').read().strip()

	dev_file = args.dev_secret
	if dev_file is None:
		dev_file = Filename(DEFAULT_CLIENT_DEV_SECRET)
	try:
		dev_secret: Optional[str] = open(dev_file, 'r').read().strip()
	except FileNotFoundError:
		dev_secret = None

	prod_file = args.prod_secret
	if prod_file is None:
		prod_file = Filename(DEFAULT_CLIENT_PROD_SECRET)
	try:
		prod_secret: Optional[str] = open(prod_file, 'r').read().strip()
	except FileNotFoundError:
		prod_secret = None
	secrets: Secrets = {
		'development': dev_secret,
		'production': prod_secret,
	}

	fetched_accounts, balances = fetchAccounts(client_id, secrets, logins)
	print(f'fetched_accounts = {fetched_accounts}')
	fetched_account_ids = set(map(lambda x: x['accountID'], fetched_accounts))
	print(f'Added account IDs: {fetched_account_ids - account_ids}')
	print(f'Removed account IDs: {account_ids - fetched_account_ids}')

	for balance in balances:
		available = str(balance['availableBalance']) if balance['availableBalance'] is not None else ''
		current = str(balance['currentBalance']) if balance['currentBalance'] is not None else ''
		limit = str(balance['limit']) if balance['limit'] is not None else ''
		print(f"{balance['accountID']},{balance['mask']},{available},{current},{limit}")

	return
	# end main()


if __name__ == '__main__':
	main()
