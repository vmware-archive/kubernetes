#!/usr/bin/env python

"""
* *******************************************************
* Copyright (c) VMware, Inc. 2014, 2016. All Rights Reserved.
* SPDX-License-Identifier: MIT
* *******************************************************
*
* DISCLAIMER. THIS PROGRAM IS PROVIDED TO YOU "AS IS" WITHOUT
* WARRANTIES OR CONDITIONS OF ANY KIND, WHETHER ORAL OR WRITTEN,
* EXPRESS OR IMPLIED. THE AUTHOR SPECIFICALLY DISCLAIMS ANY IMPLIED
* WARRANTIES OR CONDITIONS OF MERCHANTABILITY, SATISFACTORY QUALITY,
* NON-INFRINGEMENT AND FITNESS FOR A PARTICULAR PURPOSE.

THIS PROGRAM IS A MODIFIED MINIMAL VERSION OF 
https://github.com/vmware/vsphere-automation-sdk-python/blob/master/samples/vsphere/tagging/tagging_workflow.py
"""

__author__ = 'VMware, Inc.'
__copyright__ = 'Copyright 2014, 2016 VMware, Inc. All rights reserved.'
__vcenter_version__ = '6.0+'

import time

from com.vmware.cis.tagging_client import (
    Category, CategoryModel, Tag, TagAssociation)
from com.vmware.vapi.std_client import DynamicID

import samples

from samples.vsphere.common.sample_base import SampleBase
from samples.vsphere.common.vim.helpers.get_datastore_by_name import get_datastore_id


class TaggingWorkflow(SampleBase):
    """
    Demonstrates tagging CRUD operations
    Step 1: Create a Tag category.
    Step 2: Create a Tag under the category.
    Step 3: Retrieve the managed object id of an existing datastore from its name.
    Step 4: Assign the tag to the datastore.
    """
    def __init__(self):
        SampleBase.__init__(self, self.__doc__)
        self.servicemanager = None

        self.category_svc = None
        self.tag_svc = None
        self.tag_association = None

        self.category_name = None
        self.category_desc = None
        self.tag_name = None
        self.tag_desc = None

        self.datastore_name = None
        self.datastore_moid = None
        self.category_id = None
        self.tag_id = None
        self.tag_attached = False
        self.dynamic_id = None

    def _options(self):
        self.argparser.add_argument('-datastorename', '--datastorename', help='Name of the datastore to be tagged')
        self.argparser.add_argument('-categoryname', '--categoryname', help='Name of the Category to be created')
        self.argparser.add_argument('-categorydesc', '--categorydesc', help='Description of the Category to be created')
        self.argparser.add_argument('-tagname', '--tagname', help='Name of the tag to be created')
        self.argparser.add_argument('-tagdesc', '--tagdesc', help='Description of the tag to be created')

    def _setup(self):
        if self.datastore_name is None:  # for testing
            self.datastore_name = self.args.datastorename
        assert self.datastore_name is not None
        print('datastore Name: {0}'.format(self.datastore_name))

        if self.category_name is None:
            self.category_name = self.args.categoryname
        assert self.category_name is not None
        print('Category Name: {0}'.format(self.category_name))

        if self.category_desc is None:
            self.category_desc = self.args.categorydesc
        assert self.category_desc is not None
        print('Category Description: {0}'.format(self.category_desc))

        if self.tag_name is None:
            self.tag_name = self.args.tagname
        assert self.tag_name is not None
        print('Tag Name: {0}'.format(self.tag_name))

        if self.tag_desc is None:
            self.tag_desc = self.args.tagdesc
        assert self.tag_desc is not None
        print('Tag Description: {0}'.format(self.tag_desc))

        if self.servicemanager is None:
            self.servicemanager = self.get_service_manager()

        # Sample is not failing if datastorename passed is not valid
        # Validating if datastore Name passed is Valid
        print('finding the datastore {0}'.format(self.datastore_name))
        self.datastore_moid = get_datastore_id(service_manager=self.servicemanager, datastore_name=self.datastore_name)
        assert self.datastore_moid is not None
        print('Found datastore:{0} mo_id:{1}'.format(self.datastore_name, self.datastore_moid))

        self.category_svc = Category(self.servicemanager.stub_config)
        self.tag_svc = Tag(self.servicemanager.stub_config)
        self.tag_association = TagAssociation(self.servicemanager.stub_config)

    def _execute(self):
        categories = self.category_svc.list()
        for category in categories:
            catObj = self.category_svc.get(category)
            if catObj.name == self.category_name:
        	print('Found existing category: {0}. Exiting.'.format(catObj.name))
		return

        tags = self.tag_svc.list()
        for tag in tags:
            tagObj = self.tag_svc.get(tag)
            if tagObj.name == self.tag_name:
                print('Found existing tag: {0}. Exiting.'.format(tagObj.name))
                return
 
        print('creating a new tag category...')
        self.category_id = self.create_tag_category(self.category_name, self.category_desc,
                                                    CategoryModel.Cardinality.MULTIPLE)
        assert self.category_id is not None
        print('Tag category created; Id: {0}'.format(self.category_id))

        print("creating a new Tag...")
        self.tag_id = self.create_tag(self.tag_name, self.tag_desc, self.category_id)
        assert self.tag_id is not None
        print('Tag created; Id: {0}'.format(self.tag_id))

        print('Tagging the datastore {0}...'.format(self.datastore_name))
        self.dynamic_id = DynamicID(type='Datastore', id=self.datastore_moid)
        self.tag_association.attach(tag_id=self.tag_id, object_id=self.dynamic_id)
        for tag_id in self.tag_association.list_attached_tags(self.dynamic_id):
            if tag_id == self.tag_id:
                self.tag_attached = True
                break
        assert self.tag_attached
        print('Tagged datastore: {0}'.format(self.datastore_moid))

    def create_tag_category(self, name, description, cardinality):
        """create a category. User who invokes this needs create category privilege."""
        create_spec = self.category_svc.CreateSpec()
        create_spec.name = name
        create_spec.description = description
        create_spec.cardinality = cardinality
        associableTypes = set()
        create_spec.associable_types = associableTypes
        return self.category_svc.create(create_spec)

    def create_tag(self, name, description, category_id):
        """Creates a Tag"""
        create_spec = self.tag_svc.CreateSpec()
        create_spec.name = name
        create_spec.description = description
        create_spec.category_id = category_id
        return self.tag_svc.create(create_spec)

def main():
    tagging_workflow = TaggingWorkflow()
    tagging_workflow.main()


# Start program
if __name__ == '__main__':
    main()
