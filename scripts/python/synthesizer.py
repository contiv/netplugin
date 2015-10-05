#!/usr/bin/python

# synthesizer
import testbedApi
import time
import sys
import objmodel
import threading

def sleepMs(ms):
	time.sleep (ms / 1000.0);

# Get the tenant
tenant = objmodel.tenant('default')

# Create policy
tenant.newPolicy('1111111111')
tenant.newPolicy('2222222222')
tenant.newPolicy('3333333333')
tenant.newPolicy('4444444444')
tenant.newPolicy('5555555555')
tenant.newPolicy('6666666666')
tenant.newPolicy('7777777777')
tenant.newPolicy('8888888888')

# Create Groups
g0 = tenant.newGroup("0group0", networkName="private", policies=["1111111111", "2222222222", "3333333333", "4444444444", "5555555555", "6666666666", "7777777777", "8888888888"])

numGroups = 8
stepDelay = 600.0

groups = []
for grpId in range(numGroups):
	group = tenant.newGroup(str(grpId + 1) + "group" + str(grpId + 1), networkName="private", policies=['1111111111'])
	groups.append(group)

def policy_exec_thread(groupName, networkName, policyList):
	tenant.newGroup(groupName, networkName=networkName, policies=policyList)

def setGroupHeight(heights):
	for grpId in range(numGroups):
		ht = heights[grpId]
		policyList = []
		for hid in range(ht):
			pid = hid + 1
			pname = str(pid) + str(pid) + str(pid) + str(pid) + str(pid) + str(pid) + str(pid) + str(pid) + str(pid) + str(pid)
			policyList.append(pname)

		# Change group
		npThread = threading.Thread(target=policy_exec_thread, args=(str(grpId + 1) + "group" + str(grpId + 1), "private", policyList))
		npThread.start()
		# group = tenant.newGroup(str(grpId + 1) + "group" + str(grpId + 1), networkName="private", policies=policyList)

time.sleep(5)

for iter in range(3):
	setGroupHeight([2, 2, 2, 2, 2, 2, 2, 2])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([2, 2, 2, 2, 2, 2, 2, 2])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([2, 2, 2, 2, 2, 2, 2, 2])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([2, 2, 2, 2, 2, 2, 2, 2])
	sleepMs(stepDelay)
	setGroupHeight([4, 4, 4, 4, 4, 4, 4, 4])
	sleepMs(stepDelay)
	setGroupHeight([8, 8, 8, 8, 8, 8, 8, 8])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([2, 2, 2, 2, 2, 2, 2, 2])
	sleepMs(stepDelay)
	setGroupHeight([4, 4, 4, 4, 4, 4, 4, 4])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([4, 4, 4, 4, 4, 4, 4, 4])
	sleepMs(stepDelay)
	setGroupHeight([8, 8, 8, 8, 8, 8, 8, 8])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([4, 4, 4, 4, 4, 4, 4, 4])
	sleepMs(stepDelay)
	setGroupHeight([8, 8, 8, 8, 8, 8, 8, 8])
	sleepMs(stepDelay)
	setGroupHeight([1, 1, 1, 1, 1, 1, 1, 1])
	sleepMs(stepDelay)
	setGroupHeight([4, 4, 4, 4, 4, 4, 4, 4])
	sleepMs(stepDelay)
	setGroupHeight([8, 8, 8, 8, 8, 8, 8, 8])
	sleepMs(stepDelay)

time.sleep(5)

tenant.deleteGroup("0group0")
for grpId in range(numGroups):
	tenant.deleteGroup(str(grpId + 1) + "group" + str(grpId + 1))
