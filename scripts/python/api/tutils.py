# Python testing utility classes and functions
import os
import time
import traceback

def info(str):
    print "###################### " + str + " ######################"

def log(str):
    print "######## " + str

def exit(str):
    info("Test failed: " + str)
    log("Printing stacktrace of the error:")
    traceback.print_stack()
    log("Exiting...")
    time.sleep(1)
    os._exit(1)
