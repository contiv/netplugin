#!/bin/bash
netctl policy create icmpPol
netctl policy rule-add icmpPol 1 -direction=in -protocol=icmp -action=deny
netctl group create poc-net noping-epg -policy=icmpPol

