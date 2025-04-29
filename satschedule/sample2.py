# written with claude 3.7 sonnet web UI with the following prompt:
# Give an example input for the SMT solver Z3 that highlights how it can be used
# for re-scheduling a calendar to make room for a meeting? Like, if someone asks
# to meet with you and provides three possibilities that otherwise show conflicts
# with your calendar, try removing one of those conflicting meetings, and see if
# you can find a different time (during regular working hours for requester and
# requestee) for that meeting.

from z3 import *

# Create a Z3 solver
solver = Solver()

# Define time slots (hours in a work day: 9am-5pm)
TIME_SLOTS = 8
DAYS = 5  # Monday to Friday
WORKING_HOURS_START = 9  # 9am

# Creating boolean variables for each time slot in the week
# your_calendar[day][hour] = True means you have a meeting at that time
your_calendar = [[Bool(f"your_meeting_day{d}_hour{h}") for h in range(TIME_SLOTS)] for d in range(DAYS)]

# The requester's calendar (True means they're busy)
requester_calendar = [[Bool(f"requester_busy_day{d}_hour{h}") for h in range(TIME_SLOTS)] for d in range(DAYS)]

# Define your current meetings (True = busy)
# Monday meetings
solver.add(your_calendar[0][0] == True)  # Monday 9am
solver.add(your_calendar[0][1] == True)  # Monday 10am
solver.add(your_calendar[0][2] == False)  # Monday 11am
solver.add(your_calendar[0][3] == True)  # Monday 12pm
solver.add(your_calendar[0][4] == False)  # Monday 1pm
solver.add(your_calendar[0][5] == False)  # Monday 2pm
solver.add(your_calendar[0][6] == True)  # Monday 3pm
solver.add(your_calendar[0][7] == True)  # Monday 4pm

# Tuesday meetings
solver.add(your_calendar[1][0] == False)  # Tuesday 9am
solver.add(your_calendar[1][1] == True)   # Tuesday 10am - THIS IS MEETING A
solver.add(your_calendar[1][2] == True)   # Tuesday 11am
solver.add(your_calendar[1][3] == False)  # Tuesday 12pm
solver.add(your_calendar[1][4] == False)  # Tuesday 1pm
solver.add(your_calendar[1][5] == True)   # Tuesday 2pm
solver.add(your_calendar[1][6] == True)   # Tuesday 3pm
solver.add(your_calendar[1][7] == False)  # Tuesday 4pm

# Wednesday meetings
solver.add(your_calendar[2][0] == True)   # Wednesday 9am
solver.add(your_calendar[2][1] == True)   # Wednesday 10am - THIS IS MEETING B
solver.add(your_calendar[2][2] == False)  # Wednesday 11am
solver.add(your_calendar[2][3] == False)  # Wednesday 12pm
solver.add(your_calendar[2][4] == True)   # Wednesday 1pm
solver.add(your_calendar[2][5] == True)   # Wednesday 2pm
solver.add(your_calendar[2][6] == False)  # Wednesday 3pm
solver.add(your_calendar[2][7] == False)  # Wednesday 4pm

# Thursday meetings
solver.add(your_calendar[3][0] == False)  # Thursday 9am
solver.add(your_calendar[3][1] == False)  # Thursday 10am
solver.add(your_calendar[3][2] == True)   # Thursday 11am
solver.add(your_calendar[3][3] == True)   # Thursday 12pm
solver.add(your_calendar[3][4] == True)   # Thursday 1pm - THIS IS MEETING C
solver.add(your_calendar[3][5] == False)  # Thursday 2pm
solver.add(your_calendar[3][6] == False)  # Thursday 3pm
solver.add(your_calendar[3][7] == True)   # Thursday 4pm

# Friday meetings
solver.add(your_calendar[4][0] == True)   # Friday 9am
solver.add(your_calendar[4][1] == False)  # Friday 10am
solver.add(your_calendar[4][2] == False)  # Friday 11am
solver.add(your_calendar[4][3] == True)   # Friday 12pm
solver.add(your_calendar[4][4] == True)   # Friday 1pm
solver.add(your_calendar[4][5] == True)   # Friday 2pm
solver.add(your_calendar[4][6] == True)   # Friday 3pm
solver.add(your_calendar[4][7] == False)  # Friday 4pm

# Define the requester's availability (True = busy)
# Assume they're busy in these slots
for d in range(DAYS):
    for h in range(TIME_SLOTS):
        # Default is available, we'll specify busy times
        solver.add(requester_calendar[d][h] == False)

# Requester busy times
solver.add(requester_calendar[0][6] == True)  # Monday 3pm
solver.add(requester_calendar[0][7] == True)  # Monday 4pm
solver.add(requester_calendar[1][0] == True)  # Tuesday 9am
solver.add(requester_calendar[1][1] == True)  # Tuesday 10am
solver.add(requester_calendar[2][4] == True)  # Wednesday 1pm
solver.add(requester_calendar[2][5] == True)  # Wednesday 2pm
solver.add(requester_calendar[2][6] == True)  # Wednesday 3pm
solver.add(requester_calendar[3][0] == True)  # Thursday 9am
solver.add(requester_calendar[4][4] == True)  # Friday 1pm
solver.add(requester_calendar[4][5] == True)  # Friday 2pm
solver.add(requester_calendar[4][6] == True)  # Friday 3pm
solver.add(requester_calendar[4][7] == True)  # Friday 4pm

# The three possible meeting times proposed by requester (all currently have conflicts)
proposed_slots = [
    (1, 1),  # Tuesday 10am (conflicts with MEETING A)
    (2, 1),  # Wednesday 10am (conflicts with MEETING B)
    (3, 4),  # Thursday 1pm (conflicts with MEETING C)
]

# Create variables to track which meeting could be rescheduled
meeting_a_moved = Bool("meeting_a_moved")
meeting_b_moved = Bool("meeting_b_moved")
meeting_c_moved = Bool("meeting_c_moved")

# Create variables to track where each meeting might be rescheduled
# Day and hour for each rescheduled meeting
meeting_a_new_day = Int("meeting_a_new_day")
meeting_a_new_hour = Int("meeting_a_new_hour")
meeting_b_new_day = Int("meeting_b_new_day")
meeting_b_new_hour = Int("meeting_b_new_hour")
meeting_c_new_day = Int("meeting_c_new_day")
meeting_c_new_hour = Int("meeting_c_new_hour")

# New variable to determine which proposed slot to accept
use_proposal_1 = Bool("use_proposal_1")
use_proposal_2 = Bool("use_proposal_2")
use_proposal_3 = Bool("use_proposal_3")

# Constraint: Accept exactly one proposal
solver.add(
    PbEq([(use_proposal_1, 1), (use_proposal_2, 1), (use_proposal_3, 1)], 1)
)

# Constraint: Move at most one meeting
solver.add(
    PbLe([(meeting_a_moved, 1), (meeting_b_moved, 1), (meeting_c_moved, 1)], 1)
)

# If we accept proposal 1, we need to move Meeting A
solver.add(Implies(use_proposal_1, meeting_a_moved))

# If we accept proposal 2, we need to move Meeting B
solver.add(Implies(use_proposal_2, meeting_b_moved))

# If we accept proposal 3, we need to move Meeting C
solver.add(Implies(use_proposal_3, meeting_c_moved))

# Constrain new meeting times to valid values
for meeting_day, meeting_hour in [(meeting_a_new_day, meeting_a_new_hour),
                                  (meeting_b_new_day, meeting_b_new_hour),
                                  (meeting_c_new_day, meeting_c_new_hour)]:
    solver.add(And(0 <= meeting_day, meeting_day < DAYS))
    solver.add(And(0 <= meeting_hour, meeting_hour < TIME_SLOTS))

# Ensure moved meetings don't conflict with existing meetings or requester's schedule
def no_conflict_at(day, hour):
    # Original calendar without the moved meetings
    if day == 1 and hour == 1:  # Meeting A slot
        return And(Not(your_calendar[day][hour]) or meeting_a_moved, 
                  Not(requester_calendar[day][hour]))
    elif day == 2 and hour == 1:  # Meeting B slot
        return And(Not(your_calendar[day][hour]) or meeting_b_moved, 
                  Not(requester_calendar[day][hour]))
    elif day == 3 and hour == 4:  # Meeting C slot
        return And(Not(your_calendar[day][hour]) or meeting_c_moved, 
                  Not(requester_calendar[day][hour]))
    else:
        return And(Not(your_calendar[day][hour]), 
                  Not(requester_calendar[day][hour]))

# Constraint: New meeting times must be free for both parties
solver.add(Implies(meeting_a_moved, 
                  no_conflict_at(meeting_a_new_day, meeting_a_new_hour)))
solver.add(Implies(meeting_b_moved, 
                  no_conflict_at(meeting_b_new_day, meeting_b_new_hour)))
solver.add(Implies(meeting_c_moved, 
                  no_conflict_at(meeting_c_new_day, meeting_c_new_hour)))

# Make sure we don't double-book the moved meetings
solver.add(Implies(And(meeting_a_moved, meeting_b_moved),
                  Not(And(meeting_a_new_day == meeting_b_new_day,
                         meeting_a_new_hour == meeting_b_new_hour))))
solver.add(Implies(And(meeting_a_moved, meeting_c_moved),
                  Not(And(meeting_a_new_day == meeting_c_new_day,
                         meeting_a_new_hour == meeting_c_new_hour))))
solver.add(Implies(And(meeting_b_moved, meeting_c_moved),
                  Not(And(meeting_b_new_day == meeting_c_new_day,
                         meeting_b_new_hour == meeting_c_new_hour))))

# Check if there is a solution
if solver.check() == sat:
    model = solver.model()
    
    # See which proposal was accepted
    proposal_accepted = None
    if model.evaluate(use_proposal_1):
        proposal_accepted = "Proposal 1 (Tuesday 10am)"
    elif model.evaluate(use_proposal_2):
        proposal_accepted = "Proposal 2 (Wednesday 10am)"
    elif model.evaluate(use_proposal_3):
        proposal_accepted = "Proposal 3 (Thursday 1pm)"
    
    print(f"Solution found! Accepting: {proposal_accepted}")
    
    # See which meeting was moved
    if model.evaluate(meeting_a_moved):
        new_day = model.evaluate(meeting_a_new_day).as_long()
        new_hour = model.evaluate(meeting_a_new_hour).as_long()
        day_names = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
        print(f"Meeting A moved from Tuesday 10am to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00")
    elif model.evaluate(meeting_b_moved):
        new_day = model.evaluate(meeting_b_new_day).as_long()
        new_hour = model.evaluate(meeting_b_new_hour).as_long()
        day_names = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
        print(f"Meeting B moved from Wednesday 10am to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00")
    elif model.evaluate(meeting_c_moved):
        new_day = model.evaluate(meeting_c_new_day).as_long()
        new_hour = model.evaluate(meeting_c_new_hour).as_long()
        day_names = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
        print(f"Meeting C moved from Thursday 1pm to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00")
else:
    print("No solution found. Cannot reschedule meetings to accommodate any of the proposals.")
