# yahoo.py
# Trevor Pottinger
# Fri Oct 18 22:57:23 PDT 2019

class Yahoo(object):
    @classmethod
    async def gen_s_and_p_history() -> None:
        url = 'https://finance.yahoo.com/quote/%5EGSPC/history?period1=1413529200&period2=1571295600&interval=1d&filter=history&frequency=1d'

