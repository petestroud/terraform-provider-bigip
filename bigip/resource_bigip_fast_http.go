/*
Copyright 2022 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

package bigip

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/f5devcentral/go-bigip/f5teem"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var fastTmpl = "bigip-fast-templates/http"

func resourceBigipHttpFastApp() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipFastHttpAppCreate,
		ReadContext:   resourceBigipFastHttpAppRead,
		UpdateContext: resourceBigipFastHttpAppUpdate,
		DeleteContext: resourceBigipFastHttpAppDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"tenant": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of FAST HTTP application tenant.",
			},
			"application": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of FAST HTTP application.",
			},
			"virtual_server": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "foo",
						},
						"port": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "foo",
						},
					},
				},
			},
			"existing_snat_pool": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "name of an existing BIG-IP SNAT pool",
				ConflictsWith: []string{"snat_pool_address"},
			},
			"snat_pool_address": {
				Type:          schema.TypeList,
				Optional:      true,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"existing_snat_pool"},
			},
			"existing_pool": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "Select an existing BIG-IP Pool",
				ConflictsWith: []string{"pool_members", "service_discovery", "existing_monitor", "monitor"},
			},
			"pool_members": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"addresses": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  80,
						},
						"connection_limit": {
							Type:     schema.TypeInt,
							Computed: true,
							Optional: true,
						},
						"priority_group": {
							Type:     schema.TypeInt,
							Computed: true,
							Optional: true,
						},
						"share_nodes": {
							Type:     schema.TypeBool,
							Computed: true,
							Optional: true,
						},
					},
				},
				ConflictsWith: []string{"existing_pool"},
			},
			"service_discovery": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"sd_type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"sd_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"sd_aws_tag_key": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_aws_tag_val": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_aws_region": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_aws_access_key": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_aws_secret_access_key": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_address_realm": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "private",
						},
						"sd_undetectable_action": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "remove",
						},
						"sd_azure_resource_group": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"sd_azure_subscription_id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"sd_azure_resource_id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							// ConflictsWith: []string{"service_discovery.sd_azure_tag_key", "service_discovery.sd_azure_tag_val"},
						},
						"sd_azure_directory_id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							// ConflictsWith: []string{"sd_azure_tag_key", "sd_azure_tag_val"},
						},
						"sd_azure_tag_key": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
							// ConflictsWith: []string{"sd_azure_resource_id", "sd_azure_directory_id"},
						},
						"sd_azure_tag_val": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
							// ConflictsWith: []string{"sd_azure_resource_id", "sd_azure_directory_id"},
						},
						"sd_gce_tag_key": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_gce_tag_val": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
						"sd_gce_region": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
						},
					},
				},
				ConflictsWith: []string{"existing_pool"},
			},
			"load_balancing_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "none",
				ValidateFunc: validation.StringInSlice([]string{
					"dynamic-ratio-member",
					"dynamic-ratio-node",
					"fastest-app-response",
					"fastest-node",
					"least-connections-member",
					"least-connections-node",
					"least-sessions",
					"observed-member",
					"observed-node",
					"predictive-member",
					"predictive-node",
					"ratio-least-connections-member",
					"ratio-least-connections-node",
					"ratio-member",
					"ratio-node",
					"ratio-session",
					"round-robin",
					"weighted-least-connections-member",
					"weighted-least-connections-node"}, false),
			},
			"slow_ramp_time": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "none",
			},
			"existing_waf_security_policy": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"waf_security_policy"},
			},
			"waf_security_policy": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enable": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Enables Fast to WAF security policy",
						},
					},
				},
				ConflictsWith: []string{"existing_waf_security_policy"},
			},
			"endpoint_ltm_policy": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"existing_monitor": {
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "/Common/http",
				Description:   "Select an existing BIG-IP HTTPS pool monitor. Monitors are used to determine the health of the application on each server",
				ConflictsWith: []string{"existing_pool", "monitor"},
			},
			"monitor": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Use a FAST generated pool monitor.",
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"monitor_auth": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"username": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"password": {
							Type:      schema.TypeString,
							Optional:  true,
							Sensitive: true,
						},
						"interval": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Set the time between health checks, in seconds.",
						},
						"send_string": {
							Type:     schema.TypeString,
							Optional: true,
							//Default:"GET / HTTP/1.1\r\nHost: example.com\r\nConnection: Close\r\n\r\n",
							Description: "Optional data to be sent during each health check.",
						},
						"response": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				ConflictsWith: []string{"existing_monitor", "existing_pool"},
			},
			"security_log_profiles": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Existing security log profiles to enable.",
			},
			"fast_http_json": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Json payload for FAST HTTP application.",
			},
		},
	}
}

func resourceBigipFastHttpAppCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	fastJson, err := getFastHttpConfig(d)
	if err != nil {
		return diag.FromErr(err)
	}
	m.Lock()
	defer m.Unlock()
	log.Printf("[INFO] Creating HTTP FastApp config")
	userAgent := fmt.Sprintf("?userAgent=%s/%s", client.UserAgent, fastTmpl)
	tenant, app, err := client.PostFastAppBigip(fastJson, fastTmpl, userAgent)
	if err != nil {
		return diag.FromErr(err)
	}
	_ = d.Set("tenant", tenant)
	_ = d.Set("application", app)
	log.Printf("[DEBUG] ID for resource :%+v", app)
	d.SetId(d.Get("application").(string))
	var wafEnabled bool
	wafEnabled = false
	if _, ok := d.GetOk("existing_waf_security_policy"); ok {
		wafEnabled = true
	}
	// if _, ok := d.GetOk("existing_waf_security_policy"); ok {
	//  	wafEnabled = true
	// }
	if !client.Teem {
		id := uuid.New()
		uniqueID := id.String()
		assetInfo := f5teem.AssetInfo{
			Name:    "Terraform-provider-bigip",
			Version: client.UserAgent,
			Id:      uniqueID,
		}
		apiKey := os.Getenv("TEEM_API_KEY")
		teemDevice := f5teem.AnonymousClient(assetInfo, apiKey)
		f := map[string]interface{}{
			"Terraform Version": client.UserAgent,
			"application type":  "HTTP",
			"tenant":            tenant,
			"application":       app,
			"waf Enabled":       wafEnabled,
		}
		tsVer := strings.Split(client.UserAgent, "/")
		err = teemDevice.Report(f, "bigip_fast_http_app", tsVer[3])
		if err != nil {
			log.Printf("[ERROR]Sending Telemetry data failed:%v", err)
		}
	}
	return resourceBigipFastHttpAppRead(ctx, d, meta)
}

func resourceBigipFastHttpAppRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	var fastHttp bigip.FastHttpJson
	log.Printf("[INFO] Reading FastApp HTTP config")
	name := d.Id()
	tenant := d.Get("tenant").(string)
	log.Printf("[DEBUG] FAST HTTP application get call : %s", name)
	fastJson, err := client.GetFastApp(tenant, name)
	log.Printf("[DEBUG] FAST json retreived from the GET call in Read function : %s", fastJson)
	if err != nil {
		log.Printf("[ERROR] Unable to retrieve json ")
		if err.Error() == "unexpected end of JSON input" {
			log.Printf("[ERROR] %v", err)
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if fastJson == "" {
		log.Printf("[WARN] Json (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	_ = d.Set("fast_http_json", fastJson)
	err = json.Unmarshal([]byte(fastJson), &fastHttp)
	if err != nil {
		return diag.FromErr(err)
	}
	err = setFastHttpData(d, fastHttp)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceBigipFastHttpAppUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	fastJson, e := getFastHttpConfig(d)
	if e != nil {
		return diag.FromErr(e)
	}
	m.Lock()
	defer m.Unlock()
	log.Printf("[INFO] Updating FastApp Config :%s", fastJson)
	name := d.Id()
	tenant := d.Get("tenant").(string)
	err := client.ModifyFastAppBigip(fastJson, tenant, name)
	if err != nil {
		return diag.FromErr(err)
	}
	return resourceBigipFastHttpAppRead(ctx, d, meta)
}

func resourceBigipFastHttpAppDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	m.Lock()
	defer m.Unlock()
	name := d.Id()
	tenant := d.Get("tenant").(string)
	err := client.DeleteFastAppBigip(tenant, name)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

func setFastHttpData(d *schema.ResourceData, data bigip.FastHttpJson) error {
	log.Printf("My HTTP DATA:%+v", data)
	_ = d.Set("tenant", data.Tenant)
	_ = d.Set("application", data.Application)
	vsdata := make(map[string]interface{})
	vsdata["ip"] = data.VirtualAddress
	vsdata["port"] = data.VirtualPort
	_ = d.Set("virtual_server", []interface{}{vsdata})
	_ = d.Set("existing_snat_pool", data.SnatPoolName)
	_ = d.Set("snat_pool_address", data.SnatAddresses)
	_ = d.Set("security_log_profiles", data.LogProfileNames)
	_ = d.Set("existing_pool", data.PoolName)
	members := flattenFastPoolMembers(data.PoolMembers)
	_ = d.Set("pool_members", members)
	_ = d.Set("load_balancing_mode", data.LoadBalancingMode)
	if _, ok := d.GetOk("slow_ramp_time"); ok {
		_ = d.Set("slow_ramp_time", data.SlowRampTime)
	}
	_ = d.Set("existing_monitor", data.HTTPMonitor)
	if _, ok := d.GetOk("existing_waf_security_policy"); ok {
		_ = d.Set("existing_waf_security_policy", data.WafPolicyName)
	}
	if _, ok := d.GetOk("endpoint_ltm_policy"); ok {
		_ = d.Set("endpoint_ltm_policy", data.EndpointPolicyNames)
	}
	if _, ok := d.GetOk("monitor"); ok {
		if err := d.Set("monitor", []interface{}{flattenFastMonitor(data)}); err != nil {
			return fmt.Errorf("error setting monitor: %w", err)
		}
	}
	if _, ok := d.GetOk("service_discovery"); ok {
		_ = d.Set("service_discovery", flattenFastServiceDiscovery(data.ServiceDiscovery))
	}
	return nil
}

func flattenFastVirtualServers(data bigip.FastHttpJson) map[string]interface{} {
	tfMap := map[string]interface{}{}
	tfMap["ip"] = data.VirtualAddress
	tfMap["port"] = data.VirtualPort
	return tfMap
}

func flattenFastMonitor(data bigip.FastHttpJson) map[string]interface{} {
	tfMap := map[string]interface{}{}
	if data.MonitorAuth {
		tfMap["monitor_auth"] = data.MonitorAuth
	}
	tfMap["username"] = data.MonitorUsername
	tfMap["password"] = data.MonitorPassword
	if data.MonitorInterval > 0 {
		tfMap["interval"] = data.MonitorInterval
	}
	tfMap["send_string"] = data.MonitorSendString
	tfMap["response"] = data.MonitorResponse
	return tfMap
}

func flattenFastServiceDiscovery(members []bigip.ServiceDiscoverObj) []interface{} {
	att := make([]interface{}, len(members))
	for i, v := range members {
		obj := make(map[string]interface{})
		obj["sd_type"] = v.SdType
		obj["sd_port"] = v.SdPort
		if v.SdType == "aws" {
			obj["sd_aws_tag_key"] = v.SdTagKey
			obj["sd_aws_tag_val"] = v.SdTagVal
			obj["sd_address_realm"] = v.SdAddressRealm
			obj["sd_undetectable_action"] = v.SdUndetectableAction
			att[i] = obj
		}
		if v.SdType == "azure" {
			obj["sd_azure_directory_id"] = v.SdDirid
			obj["sd_azure_resource_group"] = v.SdRg
			obj["sd_azure_resource_id"] = v.SdRid
			obj["sd_azure_subscription_id"] = v.SdSid
			obj["sd_azure_tag_key"] = v.SdAzureTagKey
			obj["sd_azure_tag_val"] = v.SdAzureTagVal
			obj["sd_address_realm"] = v.SdAddressRealm
			obj["sd_undetectable_action"] = v.SdUndetectableAction
			att[i] = obj
		}
		if v.SdType == "gce" {
			obj["sd_gce_tag_key"] = v.SdTagKey
			obj["sd_gce_tag_val"] = v.SdTagVal
			obj["sd_address_realm"] = v.SdAddressRealm
			obj["sd_undetectable_action"] = v.SdUndetectableAction
			att[i] = obj
		}
	}
	return att
}

func flattenFastPoolMembers(members []bigip.FastHttpPool) []interface{} {
	att := make([]interface{}, len(members))
	for i, v := range members {
		obj := make(map[string]interface{})
		if len(v.ServerAddresses) > 0 {
			obj["addresses"] = v.ServerAddresses
		}
		obj["port"] = v.ServicePort
		if v.ConnectionLimit > 0 {
			obj["connection_limit"] = v.ConnectionLimit
		}
		obj["priority_group"] = v.PriorityGroup
		if v.ShareNodes {
			obj["share_nodes"] = v.ShareNodes
		}
		att[i] = obj
	}
	return att
}

func getFastHttpConfig(d *schema.ResourceData) (string, error) {
	httpJson := &bigip.FastHttpJson{
		Tenant:      d.Get("tenant").(string),
		Application: d.Get("application").(string),
	}
	httpJson.TlsServerEnable = false
	httpJson.TlsServerProfileCreate = false
	if v, ok := d.GetOk("virtual_server"); ok {
		vL := v.([]interface{})
		for _, v := range vL {
			httpJson.VirtualAddress = v.(map[string]interface{})["ip"].(string)
			httpJson.VirtualPort = v.(map[string]interface{})["port"]
		}
	}

	httpJson.PoolEnable = false
	if v, ok := d.GetOk("existing_pool"); ok {
		httpJson.PoolEnable = true
		httpJson.PoolName = v.(string)
		httpJson.MakePool = false
	}
	if p, ok := d.GetOk("pool_members"); ok {
		httpJson.PoolEnable = true
		httpJson.MakePool = true
		log.Printf("[DEBUG] Adding Pool Members:%+v", p)
		var members []bigip.FastHttpPool
		for _, r := range p.(*schema.Set).List() {
			log.Printf("[DEBUG] Pool Members:%+v and Type :%T", r, r)
			memberConfig := bigip.FastHttpPool{}
			var serAdd []string
			for _, addr := range r.(map[string]interface{})["addresses"].([]interface{}) {
				serAdd = append(serAdd, addr.(string))
			}
			memberConfig.ServerAddresses = serAdd
			memberConfig.ServicePort = r.(map[string]interface{})["port"].(int)
			if s, ok := r.(map[string]interface{})["connection_limit"].(int); ok {
				memberConfig.ConnectionLimit = s
			}
			if s, ok := r.(map[string]interface{})["priority_group"].(int); ok {
				memberConfig.PriorityGroup = s
			}
			if s, ok := r.(map[string]interface{})["share_nodes"].(bool); ok {
				memberConfig.ShareNodes = s
			}
			members = append(members, memberConfig)
		}
		httpJson.PoolMembers = members
	}

	if p, ok := d.GetOk("service_discovery"); ok {
		httpJson.SdEnable = true
		httpJson.PoolEnable = true
		httpJson.MakePool = true
		var sdObjs []bigip.ServiceDiscoverObj
		for _, r := range p.(*schema.Set).List() {
			sdObj := bigip.ServiceDiscoverObj{}
			sdObj.SdType = r.(map[string]interface{})["sd_type"].(string)
			sdObj.SdPort = r.(map[string]interface{})["sd_port"].(int)
			sdObj.SdAddressRealm = r.(map[string]interface{})["sd_address_realm"].(string)
			if sdObj.SdType == "aws" {
				if r.(map[string]interface{})["sd_aws_tag_key"].(string) == "" || r.(map[string]interface{})["sd_aws_tag_val"].(string) == "" {
					return "", fmt.Errorf("'sd_aws_tag_key' and 'sd_aws_tag_val' must be specified for aws service discovery")
				}
				sdObj.SdTagKey = r.(map[string]interface{})["sd_aws_tag_key"].(string)
				sdObj.SdTagVal = r.(map[string]interface{})["sd_aws_tag_val"].(string)
				sdObj.SdAwsRegion = r.(map[string]interface{})["sd_aws_region"].(string)
				sdObj.SdAccessKeyId = r.(map[string]interface{})["sd_aws_access_key"].(string)
				sdObj.SdSecretAccessKey = r.(map[string]interface{})["sd_aws_secret_access_key"].(string)
				sdObj.SdUndetectableAction = r.(map[string]interface{})["sd_undetectable_action"].(string)
				sdObjs = append(sdObjs, sdObj)
			}
			if sdObj.SdType == "azure" {
				if r.(map[string]interface{})["sd_azure_resource_group"].(string) == "" || r.(map[string]interface{})["sd_azure_subscription_id"].(string) == "" {
					return "", fmt.Errorf("'sd_azure_resource_group' and 'sd_azure_subscription_id' must be specified for azure service discovery")
				}
				sdObj.SdRg = r.(map[string]interface{})["sd_azure_resource_group"].(string)
				sdObj.SdSid = r.(map[string]interface{})["sd_azure_subscription_id"].(string)
				sdObj.SdRtype = "tag"
				sdObj.SdUseManagedIdentity = true
				if r.(map[string]interface{})["sd_azure_tag_key"].(string) != "" && r.(map[string]interface{})["sd_azure_tag_val"].(string) != "" {
					sdObj.SdAzureTagKey = r.(map[string]interface{})["sd_azure_tag_key"].(string)
					sdObj.SdAzureTagVal = r.(map[string]interface{})["sd_azure_tag_val"].(string)
				} else if r.(map[string]interface{})["sd_azure_resource_id"].(string) != "" {
					sdObj.SdRid = r.(map[string]interface{})["sd_azure_resource_id"].(string)
					// sdObj.SdDirid = r.(map[string]interface{})["sd_azure_directory_id"].(string)
				}
				sdObj.SdUndetectableAction = r.(map[string]interface{})["sd_undetectable_action"].(string)
				sdObjs = append(sdObjs, sdObj)
			}
			if sdObj.SdType == "gce" {
				if r.(map[string]interface{})["sd_gce_tag_key"].(string) == "" || r.(map[string]interface{})["sd_gce_tag_val"].(string) == "" || r.(map[string]interface{})["sd_gce_region"].(string) == "" {
					return "", fmt.Errorf("'sd_gce_tag_key' , 'sd_gce_tag_val' and 'sd_gce_region' must be specified for GCE service discovery")
				}
				sdObj.SdTagKey = r.(map[string]interface{})["sd_gce_tag_key"].(string)
				sdObj.SdTagVal = r.(map[string]interface{})["sd_gce_tag_val"].(string)
				sdObj.SdRegion = r.(map[string]interface{})["sd_gce_region"].(string)
				sdObj.SdUndetectableAction = r.(map[string]interface{})["sd_undetectable_action"].(string)
				sdObjs = append(sdObjs, sdObj)
			}
		}
		httpJson.ServiceDiscovery = sdObjs
	}

	httpJson.SnatEnable = true
	httpJson.SnatAutomap = true
	httpJson.WafPolicyEnable = false
	httpJson.MakeSnatPool = false
	if v, ok := d.GetOk("existing_snat_pool"); ok {
		httpJson.SnatPoolName = v.(string)
		httpJson.SnatEnable = true
		httpJson.SnatAutomap = false
		httpJson.MakeSnatPool = false
	}
	if s, ok := d.GetOk("snat_pool_address"); ok {
		httpJson.SnatEnable = true
		httpJson.SnatAutomap = false
		httpJson.MakeSnatPool = true
		var snatAdd []string
		for _, addr := range s.([]interface{}) {
			snatAdd = append(snatAdd, addr.(string))
		}
		httpJson.SnatAddresses = snatAdd
	}
	if s, ok := d.GetOk("endpoint_ltm_policy"); ok {
		var endptPolicy []string
		for _, policy := range s.([]interface{}) {
			endptPolicy = append(endptPolicy, policy.(string))
		}
		httpJson.EndpointPolicyNames = endptPolicy
	}
	if v, ok := d.GetOk("existing_waf_security_policy"); ok {
		httpJson.WafPolicyEnable = true
		httpJson.WafPolicyName = v.(string)
		httpJson.AsmLoggingEnable = true
	}
	if v, ok := d.GetOk("waf_security_policy"); ok {
		httpJson.WafPolicyEnable = true
		httpJson.MakeWafpolicy = true
		httpJson.AsmLoggingEnable = true
		// httpJson.WafPolicyName = ""
		wafPol := v.([]interface{})
		for _, vv := range wafPol {
			log.Printf("[DEBUG] waf_secu policy:%+v", vv)
		}
	}
	httpJson.MonitorEnable = false
	httpJson.MakeMonitor = false
	if v, ok := d.GetOk("existing_monitor"); ok {
		httpJson.HTTPMonitor = v.(string)
		httpJson.MonitorEnable = true
		httpJson.MakeMonitor = false
	}
	if s, ok := d.GetOk("monitor"); ok {
		log.Printf("[DEBUG] monitor:%+v", s)
		httpJson.MonitorEnable = true
		httpJson.MakeMonitor = true
		sL := s.([]interface{})
		for _, v := range sL {
			mon := v.(map[string]interface{})
			if auth, ok := mon["monitor_auth"].(bool); ok {
				if auth {
					if u, ok := mon["username"].(string); ok {
						httpJson.MonitorUsername = u
					} else {
						return "", fmt.Errorf("the 'username' must be specified if 'monitor_auth' is %t", auth)
					}
					if p, ok := mon["password"].(string); ok {
						httpJson.MonitorPassword = p
					} else {
						return "", fmt.Errorf("the 'password' must be specified if 'monitor_auth' is %t", auth)
					}
				}
				httpJson.MonitorAuth = auth
			}
			if e, ok := mon["interval"].(int); ok {
				httpJson.MonitorInterval = e
			}
			if e, ok := mon["send_string"].(string); ok {
				httpJson.MonitorSendString = e
			}
			if e, ok := mon["response"].(string); ok {
				httpJson.MonitorResponse = e
			}
		}
	}
	if p, ok := d.GetOk("load_balancing_mode"); ok {
		httpJson.LoadBalancingMode = p.(string)
	}
	if p, ok := d.GetOk("slow_ramp_time"); ok {
		httpJson.SlowRampTime = p.(int)
	}
	if s, ok := d.GetOk("security_log_profiles"); ok {
		httpJson.AsmLoggingEnable = true
		var logProfiles []string
		for _, logProfile := range s.([]interface{}) {
			logProfiles = append(logProfiles, logProfile.(string))
		}
		httpJson.LogProfileNames = logProfiles
	}
	data, err := json.Marshal(httpJson)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
