/*
Original work from https://github.com/DealerDotCom/terraform-provider-bigip
Modifications Copyright 2022 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file,You can obtain one at https://mozilla.org/MPL/2.0/.
*/
package bigip

import (
	"fmt"
	"log"
	"strings"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

func resourceBigipVcmpGuest() *schema.Resource {
	return &schema.Resource{
		Create: resourceBigipVcmpGuestCreate,
		Update: resourceBigipVcmpGuestUpdate,
		Read:   resourceBigipVcmpGuestRead,
		Delete: resourceBigipVcmpGuestDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the vCMP guest instance.",
			},
			"full_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Resource name including prepended partition path.",
			},
			"initial_image": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The base software release ISO image file for installing the TMOS hypervisor instance.",
			},
			"initial_hotfix": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The hotfix ISO image file which is applied on top of the base image.",
			},
			"vlans": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "VLANs the guest uses to communicate with other guests, the host, and with the external network.",
			},
			"mgmt_network": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The method by which the management address is used in the vCMP guest.",
				ValidateFunc: validation.StringInSlice([]string{
					"bridged",
					"isolated",
					"host-only",
				}, false),
			},
			"mgmt_address": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The IP address and subnet or subnet mask you use to access the guest when you want to manage a module running within the guest.",
			},
			"mgmt_route": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The gateway address for the mgmt_address.",
			},
			"state": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The state of the vCMP guest on the system.",
				ValidateFunc: validation.StringInSlice([]string{
					"configured",
					"provisioned",
					"deployed",
				}, false),
			},
			"cores_per_slot": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "The number of cores the system allocates to the guest.",
			},
			"number_of_slots": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "The number of slots for the system to use when creating the guest.",
			},
			"min_number_of_slots": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "The minimum number of slots the guest must be assigned to in order to deploy.",
			},
			"allowed_slots": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Contains those slots to which the guest is allowed to be assigned.",
			},
			"delete_virtual_disk": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Indicates if virtual disk associated with vCMP guest should be removed during remove operation.",
			},
			"virtual_disk": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Virtual disk associated with vCMP guest.",
			},
		},
	}
}

func resourceBigipVcmpGuestCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)
	name := d.Get("name").(string)
	log.Println("[INFO] Creating vCMP Guest: " + name)

	mgmt := d.Get("mgmt_network").(string)
	ip := d.Get("mgmt_address").(string)
	if mgmt == "bridged" && ip == "" {
		return fmt.Errorf("the mgmt_address must be provided if mgmt_network is set to bridged")
	}
	p := dataToVcmp(name, d)
	err := client.CreateVcmpGuest(&p)
	if err != nil {
		log.Printf("[ERROR] Unable to Create vCMP Guest  (%s) (%v) ", name, err)
		return err
	}
	d.SetId(name)
	return resourceBigipVcmpGuestRead(d, meta)
}

func dataToVcmp(name string, d *schema.ResourceData) bigip.VcmpGuest {
	var p bigip.VcmpGuest
	p.Name = name
	p.FullPath = d.Get("full_path").(string)
	p.InitialImage = d.Get("initial_image").(string)
	p.InitialHotfix = d.Get("initial_hotfix").(string)
	p.ManagementGw = d.Get("mgmt_route").(string)
	p.Vlans = listToStringSlice(d.Get("vlans").([]interface{}))
	p.AllowedSlots = listToIntSlice(d.Get("allowed_slots").([]interface{}))
	p.Slots = d.Get("number_of_slots").(int)
	p.MinSlots = d.Get("min_number_of_slots").(int)
	p.CoresPerSlot = d.Get("cores_per_slot").(int)
	p.State = d.Get("state").(string)
	p.ManagementNetwork = d.Get("mgmt_network").(string)
	p.ManagementIp = d.Get("mgmt_address").(string)
	return p
}

func resourceBigipVcmpGuestRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)
	name := d.Id()

	log.Printf("[INFO] Fetching vCMP Guest:%+v", name)
	p, err := client.GetVcmpGuest(name)
	if err != nil {
		log.Printf("[ERROR] Unable to Retrieve vCMP Guest  (%s) (%v) ", name, err)
		d.SetId("")
		return err
	}
	if p == nil {
		log.Printf("[WARN] vCMP Guest (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	return vcmpToData(p, d)
}

func vcmpToData(p *bigip.VcmpGuest, d *schema.ResourceData) error {
	_ = d.Set("name", p.FullPath)
	if err := d.Set("initial_image", p.InitialImage); err != nil {
		return fmt.Errorf("[DEBUG] Error saving InitialImage to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("initial_hotfix", p.InitialHotfix); err != nil {
		return fmt.Errorf("[DEBUG] Error saving InitialHotfix to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("vlans", p.Vlans); err != nil {
		return fmt.Errorf("[DEBUG] Error saving Vlans to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("mgmt_network", p.ManagementNetwork); err != nil {
		return fmt.Errorf("[DEBUG] Error saving ManagementNetwork to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("mgmt_address", p.ManagementIp); err != nil {
		return fmt.Errorf("[DEBUG] Error saving ManagementIp to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("mgmt_route", p.ManagementGw); err != nil {
		return fmt.Errorf("[DEBUG] Error saving ManagementGw to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("allowed_slots", p.AllowedSlots); err != nil {
		return fmt.Errorf("[DEBUG] Error saving AllowedSlots to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("number_of_slots", p.Slots); err != nil {
		return fmt.Errorf("[DEBUG] Error saving Slots to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("min_number_of_slots", p.MinSlots); err != nil {
		return fmt.Errorf("[DEBUG] Error saving MinSlots to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("cores_per_slot", p.CoresPerSlot); err != nil {
		return fmt.Errorf("[DEBUG] Error saving CoresPerSlot to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("state", p.State); err != nil {
		return fmt.Errorf("[DEBUG] Error saving State to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	if err := d.Set("virtual_disk", p.VirtualDisk); err != nil {
		return fmt.Errorf("[DEBUG] Error saving VirtualDisk to state for vCMP Guest  (%s): %s", d.Id(), err)
	}
	return nil
}

func resourceBigipVcmpGuestUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)
	name := d.Id()
	log.Printf("[INFO] Updating vCMP Guest:%+v", name)
	p := dataToVcmp(name, d)
	err := client.UpdateVcmpGuest(name, &p)
	if err != nil {
		log.Printf("[ERROR] Unable to Retrieve vCMP Guest  (%s) (%v) ", name, err)
		return err
	}
	return resourceBigipVcmpGuestRead(d, meta)
}

func resourceBigipVcmpGuestDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)
	name := d.Id()
	log.Printf("[INFO] Deleting vCMP Guest :%+v", name)
	err := client.DeleteVcmpGuest(name)
	if err != nil {
		log.Printf("[ERROR] Unable to Delete vCMP Guest  (%s) (%v) ", name, err)
		return err
	}
	disk, ok := d.GetOk("virtual_disk")
	if d.Get("delete_virtual_disk").(bool) && ok {
		err := deleteVirtualDisk(d, meta)
		if err != nil {
			log.Printf("[ERROR] Unable to Delete vCMP virtual disk  (%s) (%v) ", disk, err)
			return err
		}
	}

	d.SetId("")
	return nil
}

func deleteVirtualDisk(d *schema.ResourceData, meta interface{}) error {
	diskName, _ := d.Get("virtual_disk").(string)
	client := meta.(*bigip.BigIP)
	virtualDisks, err := client.GetVcmpDisks()
	if err != nil {
		return fmt.Errorf("error retrieving vCMP virtual disks: %v", err)
	}

	for _, disk := range virtualDisks.Disks {
		if strings.HasPrefix(disk.Name, diskName) {
			name := strings.Replace(disk.Name, "/", "~", 1)
			err := client.DeleteVcmpDisk(name)
			if err != nil {
				return fmt.Errorf("error deleting vCMP virtual disk: %v %v", diskName, err)
			}
		} else {
			return fmt.Errorf("cannot find vCMP virtual disk: %v ", diskName)
		}

	}
	return nil
}
