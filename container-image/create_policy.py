#!/usr/bin/env python
from pyVmomi import vim, pbm, VmomiSupport
from pyVim.connect import SmartConnect, Disconnect

import atexit
import argparse
import ast
import getpass
import sys
import ssl

"""
Example of using Storage Policy Based Management (SPBM) API
to update an existing VM Storage Policy.

Required Prviledge: Profile-driven storage update

This program is a modified version of 

https://github.com/vmware/pyvmomi-community-samples/blob/master/samples/update_vm_storage_policy.py
"""

# retrieve SPBM API endpoint
def GetPbmConnection(vpxdStub):
    import Cookie
    import pyVmomi
    sessionCookie = vpxdStub.cookie.split('"')[1]
    httpContext = VmomiSupport.GetHttpContext()
    cookie = Cookie.SimpleCookie()
    cookie["vmware_soap_session"] = sessionCookie
    httpContext["cookies"] = cookie
    VmomiSupport.GetRequestContext()["vcSessionCookie"] = sessionCookie
    hostname = vpxdStub.host.split(":")[0]
    pbmStub = pyVmomi.SoapStubAdapter(
        host=hostname,
        version="pbm.version.version1",
        path="/pbm/sdk",
        poolSize=0,
        sslContext=ssl._create_unverified_context())
    pbmSi = pbm.ServiceInstance("ServiceInstance", pbmStub)
    pbmContent = pbmSi.RetrieveContent()

    return (pbmSi, pbmContent)


# Create required SPBM Capability object from python dict
def _dictToCapability(d):
    ciList = []

    for k, v in d.iteritems():
        namespace = k.split('.')[0]
        id = constraint_id = k.split('.')[1]
        pi = pbm.capability.PropertyInstance(
                                id=constraint_id,
                                value=v
        )

        if k.split('.')[0] == 'tag':
            namespace = 'http://www.vmware.com/storage/tag'
            constraint_id = 'com.vmware.storage.tag.cluster.property'
            values = pbm.capability.types.DiscreteSet()
            values.values.append(v)
            pi.id = constraint_id
            pi.value = values

        ciList.append(
            pbm.capability.CapabilityInstance(
                id=pbm.capability.CapabilityMetadata.UniqueId(
                    namespace = namespace,
                    id=id
                ),
                constraint=[
                    pbm.capability.ConstraintInstance(
                        propertyInstance=[pi]
                    )
                ]
            )
        )
    return ciList


# Create VM Storage Policy
def CreateProfile(pm, rules, name):
    pm.PbmCreate(
        createSpec=pbm.profile.CapabilityBasedProfileCreateSpec(
            description=None,
            name=name,
            resourceType=pbm.profile.ResourceType(resourceType="STORAGE"),
            constraints=pbm.profile.SubProfileCapabilityConstraints(
                subProfiles=[
                    pbm.profile.SubProfileCapabilityConstraints.SubProfile(
                        name="Object",
                        capability=_dictToCapability(rules)
                    )
                ]
            )
        )
    )


def GetArgs():
    """
    Supports the command-line arguments listed below.
    """
    parser = argparse.ArgumentParser(
        description='Process args for VSAN SDK sample application')
    parser.add_argument('-s', '--host', required=True, action='store',
                        help='Remote host to connect to')
    parser.add_argument('-o', '--port', type=int, default=443, action='store',
                        help='Port to connect on')
    parser.add_argument('-u', '--user', required=True, action='store',
                        help='User name to use when connecting to host')
    parser.add_argument('-p', '--password', required=False, action='store',
                        help='Password to use when connecting to host')
    parser.add_argument('-n', '--policy-name', required=True, action='store',
                        help='VM Storage Policy ID')
    parser.add_argument('-r', '--policy-rule', required=True, action='store',
                        help="VM Storage Policy Rule encoded as dictionary"
                        "example:"
                        " \"{\'VSAN.hostFailuresToTolerate\':1,"
                        "    \'VSAN.stripeWidth\':2,"
                        "    \'VSAN.forceProvisioning\':False}\"")
    args = parser.parse_args()
    return args


# Start program
def main():
    args = GetArgs()
    if args.password:
        password = args.password
    else:
        password = getpass.getpass(prompt='Enter password for host %s and '
                                   'user %s: ' % (args.host, args.user))

    si = SmartConnect(host=args.host,
                      user=args.user,
                      pwd=password,
                      port=int(args.port),
                      sslContext=ssl._create_unverified_context())

    atexit.register(Disconnect, si)

    # Connect to SPBM Endpoint
    pbmSi, pbmContent = GetPbmConnection(si._stub)

    pm = pbmContent.profileManager
    profileIds = pm.PbmQueryProfile(
        resourceType=pbm.profile.ResourceType(resourceType="STORAGE"),
        profileCategory="REQUIREMENT"
    )

    profiles = []
    if len(profileIds) > 0:
        profiles = pm.PbmRetrieveContent(profileIds=profileIds)

    # Attempt to find profile name given by user
    for profile in profiles:
        if profile.name == args.policy_name:
            print("Policy with name '%s' already exists" % (args.policy_name))
            return

    #Convert string to dict
    vmPolicyRules = ast.literal_eval(args.policy_rule)

    print("Creating VM Storage Policy %s with %s ..." % (
        args.policy_name, args.policy_rule))
    CreateProfile(pm, vmPolicyRules, args.policy_name)


# Start program
if __name__ == "__main__":
    main()
