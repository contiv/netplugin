# Python testing utility classes and functions
import os
import time
import traceback

def info(str):
    print "###################### " + str + " ######################"

def log(str):
    print "######## " + str

def exit(str):
    print "\n\n\n\n\n"
    info("Test failed: " + str)
    print "\n\n\n\n\n"
    log("Printing stacktrace of the error:")
    traceback.print_stack()
    log("Exiting...")
    time.sleep(1)
    os._exit(1)
