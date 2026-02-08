# transaction_analyzer.py
# Sat Feb  7 15:39:46 PST 2026
# Trevor Pottinger

import os
from typing import Optional

import pandas as pd

DATA_DIR = os.path.join(os.path.dirname(__file__), "data")
CSV_FILE = os.path.join(DATA_DIR, "Transactions_2026-02-07T15-09-43.csv")

# ---------------------------------------------------------------------------
# Category mapping: source CSV categories -> our 14 target categories
# Anything not mapped here falls into "other"
# ---------------------------------------------------------------------------
CATEGORY_MAP: dict[str, str] = {
    # credit_card – payments between your own accounts to pay CC bills
    "Credit Card Payment": "credit_card",
    # donation
    "Charity": "donation",
    # groceries
    "Groceries": "groceries",
    # gym / fitness
    "Fitness": "gym",
    # loan
    "Loan Repayment": "loan",
    "Mortgage": "loan",
    # rent
    "Rent": "rent",
    # restaurants (fold coffee shops in here too)
    "Restaurants & Bars": "restaurants",
    "Coffee Shops": "restaurants",
    # service – professional / personal services
    "Personal": "service",
    "Medical": "service",
    "Dentist": "service",
    "Nanny": "service",
    "Child Care": "service",
    "House work": "service",
    # shopping
    "Shopping": "shopping",
    "Clothing": "shopping",
    "Furniture & Housewares": "shopping",
    # subscription – recurring digital / media / membership charges
    "Entertainment & Recreation": "subscription",
    "Internet & Cable": "subscription",
    "Phone": "subscription",
    "Education": "subscription",
    # tax
    "Taxes": "tax",
    # transfer – money moving between accounts, investments, income
    "Transfer": "transfer",
    "Paychecks": "transfer",
    "Other Income": "transfer",
    "Interest": "transfer",
    # travel
    "Travel & Vacation": "travel",
    "Taxi & Ride Shares": "travel",
    "Parking & Tolls": "travel",
    "Gas": "travel",
    # utility
    "Gas & Electric": "utility",
    "Water": "utility",
    "Business Utilities & Communication": "utility",
}

# ---------------------------------------------------------------------------
# Merchant overrides: when the CSV category is wrong or too generic,
# force a specific target category based on merchant name (lowercased substring).
# Checked BEFORE the category map.
# ---------------------------------------------------------------------------
MERCHANT_OVERRIDES: dict[str, str] = {
    "bellevue club": "gym",
    "proclub": "gym",
    "pro club": "gym",
    "arbor day foundation": "donation",
    "ziply fiber": "subscription",
    "t-mobile": "subscription",
    "sirius": "subscription",
    "netflix": "subscription",
    "youtube": "subscription",
    "new york times": "subscription",
    "monarch money": "subscription",
    "home assistant": "subscription",
    "amazon web services": "utility",
    "google workspace": "subscription",
    "oura ring": "subscription",
    "pets best insurance": "subscription",
    "the standard": "subscription",
    "trustmark benefit": "subscription",
    "annual fee": "credit_card",
    "internal revenue service": "tax",
    "city of kirkland": "utility",  # water bill
    "puget sound energy": "utility",
    "evergreen electric": "utility",
    "zenbusiness": "service",
    "bright horizons": "service",
}


def load_transactions(path: str = CSV_FILE) -> pd.DataFrame:
    """Load and clean the transactions CSV."""
    df = pd.read_csv(path, parse_dates=["Date"])
    # Drop rows that are clearly CSV-parse artifacts (shifted columns)
    df = df.dropna(subset=["Date", "Amount"])
    df["Amount"] = pd.to_numeric(df["Amount"], errors="coerce")
    df = df.dropna(subset=["Amount"])
    return df


def classify(row: pd.Series) -> str:
    """Return one of the 14 target categories (or 'other')."""
    merchant = str(row.get("Merchant", "")).lower()
    category = str(row.get("Category", ""))

    # 1. Check merchant overrides first
    for key, target in MERCHANT_OVERRIDES.items():
        if key in merchant:
            return target

    # 2. Fall back to category map
    return CATEGORY_MAP.get(category, "other")


def add_classifications(df: pd.DataFrame) -> pd.DataFrame:
    """Add a 'target_category' column."""
    df = df.copy()
    df["target_category"] = df.apply(classify, axis=1)
    return df


# ---------------------------------------------------------------------------
# Recurring / variable detection
# ---------------------------------------------------------------------------
def detect_recurrence(df: pd.DataFrame) -> pd.DataFrame:
    """
    For each merchant, determine if it's recurring (fixed amount at a
    regular interval) or variable.

    Returns a DataFrame with one row per merchant containing:
      - merchant, target_category, occurrences, total_spent
      - avg_amount, std_amount
      - avg_days_between (mean gap between consecutive transactions)
      - pattern: 'monthly', 'quarterly', 'yearly', or 'variable'
      - amount_type: 'fixed' or 'variable'
    """
    # Only look at expenses (negative amounts)
    expenses = df[df["Amount"] < 0].copy()
    expenses["abs_amount"] = expenses["Amount"].abs()

    records = []
    for merchant, group in expenses.groupby("Merchant"):
        target_cat = group["target_category"].mode().iloc[0] if len(group) > 0 else "other"
        total = group["abs_amount"].sum()
        avg_amt = group["abs_amount"].mean()
        std_amt = group["abs_amount"].std() if len(group) > 1 else 0.0

        # Compute average days between transactions
        dates = group["Date"].sort_values()
        if len(dates) > 1:
            gaps = dates.diff().dropna().dt.days
            avg_gap = gaps.mean()
        else:
            avg_gap = None

        # Classify the frequency pattern
        pattern = _classify_frequency(avg_gap, len(group))
        # Classify amount stability: "fixed" if std < 5% of mean (and > 1 occurrence)
        if len(group) > 1 and avg_amt > 0:
            cv = std_amt / avg_amt  # coefficient of variation
            amount_type = "fixed" if cv < 0.05 else "variable"
        else:
            amount_type = "variable"

        records.append({
            "merchant": merchant,
            "target_category": target_cat,
            "occurrences": len(group),
            "total_spent": round(total, 2),
            "avg_amount": round(avg_amt, 2),
            "std_amount": round(std_amt, 2),
            "avg_days_between": round(avg_gap, 1) if avg_gap is not None else None,
            "pattern": pattern,
            "amount_type": amount_type,
        })

    return pd.DataFrame(records).sort_values("total_spent", ascending=False)


def _classify_frequency(avg_gap: Optional[float], count: int) -> str:
    """Heuristic: bucket average gap into monthly/quarterly/yearly/variable."""
    if avg_gap is None or count < 2:
        return "one-time"
    if 20 <= avg_gap <= 40:
        return "monthly"
    if 80 <= avg_gap <= 110:
        return "quarterly"
    if 330 <= avg_gap <= 400:
        return "yearly"
    return "variable"


# ---------------------------------------------------------------------------
# Summary / reporting
# ---------------------------------------------------------------------------
def spending_summary(df: pd.DataFrame) -> pd.DataFrame:
    """Total and monthly-average spend per target category (expenses only)."""
    expenses = df[df["Amount"] < 0].copy()
    expenses["abs_amount"] = expenses["Amount"].abs()
    expenses["month"] = expenses["Date"].dt.to_period("M")

    n_months = expenses["month"].nunique()

    summary = (
        expenses
        .groupby("target_category")["abs_amount"]
        .agg(["sum", "count", "mean"])
        .rename(columns={"sum": "total_spent", "count": "num_transactions", "mean": "avg_per_txn"})
    )
    summary["monthly_avg"] = (summary["total_spent"] / n_months).round(2)
    summary["total_spent"] = summary["total_spent"].round(2)
    summary["avg_per_txn"] = summary["avg_per_txn"].round(2)
    return summary.sort_values("total_spent", ascending=False)


def print_report(df: pd.DataFrame) -> None:
    """Print a human-readable summary to stdout."""
    df = add_classifications(df)

    print("=" * 70)
    print("SPENDING SUMMARY BY CATEGORY")
    print("=" * 70)
    summary = spending_summary(df)
    # Determine number of months for the header
    expenses = df[df["Amount"] < 0]
    months = expenses["Date"].dt.to_period("M")
    n_months = months.nunique()
    date_range = f"{df['Date'].min().date()} to {df['Date'].max().date()}"
    print(f"Date range: {date_range} ({n_months} months)\n")

    for cat, row in summary.iterrows():
        print(f"  {cat:<16s}  ${row['total_spent']:>12,.2f}  "
              f"({int(row['num_transactions']):>5d} txns, "
              f"${row['monthly_avg']:>10,.2f}/mo avg)")

    grand_total = summary["total_spent"].sum()
    grand_monthly = summary["monthly_avg"].sum()
    print(f"  {'TOTAL':<16s}  ${grand_total:>12,.2f}  "
          f"{'':>14s}"
          f"${grand_monthly:>10,.2f}/mo avg")

    print()
    print("=" * 70)
    print("RECURRING MERCHANTS (monthly/quarterly/yearly, 3+ occurrences)")
    print("=" * 70)
    recurrence = detect_recurrence(df)
    recurring = recurrence[
        recurrence["pattern"].isin(["monthly", "quarterly", "yearly"])
        & (recurrence["occurrences"] >= 3)
    ].copy()

    for pattern in ["monthly", "quarterly", "yearly"]:
        subset = recurring[recurring["pattern"] == pattern]
        if subset.empty:
            continue
        print(f"\n  --- {pattern.upper()} ---")
        for _, r in subset.iterrows():
            fixed_label = "FIXED" if r["amount_type"] == "fixed" else "  var"
            print(f"    {r['merchant']:<35s} {r['target_category']:<14s} "
                  f"${r['avg_amount']:>10,.2f}  [{fixed_label}]  "
                  f"({r['occurrences']} txns)")

    print()
    print("=" * 70)
    print("TOP 20 VARIABLE MERCHANTS BY TOTAL SPEND")
    print("=" * 70)
    variable = recurrence[
        (recurrence["amount_type"] == "variable")
        & (recurrence["occurrences"] >= 3)
    ].head(20)
    for _, r in variable.iterrows():
        print(f"  {r['merchant']:<35s} {r['target_category']:<14s} "
              f"${r['total_spent']:>12,.2f}  "
              f"(avg ${r['avg_amount']:>8,.2f}, {r['occurrences']} txns)")


if __name__ == "__main__":
    df = load_transactions()
    print_report(df)
