// SPDX-License-Identifier: 0BSD

use std::ffi::CString;

use crate::error::{map_code, Error, Result};
use crate::ffi::{self, HASH_LEN};
use crate::identity::Identity;

pub struct Node {
    handle: u64,
}

impl Node {
    pub fn create(config_path: &str) -> Result<Self> {
        let c = CString::new(config_path).map_err(|_| Error::InvalidArg)?;
        let handle = unsafe { ffi::rrc_node_create(c.as_ptr()) };
        if handle == 0 {
            return Err(Error::Internal);
        }
        Ok(Self { handle })
    }

    pub fn start(&self) -> Result<()> {
        map_code(unsafe { ffi::rrc_node_start(self.handle) })
    }

    pub fn stop(&self) -> Result<()> {
        map_code(unsafe { ffi::rrc_node_stop(self.handle) })
    }

    pub fn set_identity(&self, identity: &Identity) -> Result<()> {
        map_code(unsafe { ffi::rrc_node_set_identity(self.handle, identity.handle()) })
    }

    pub fn add_udp_interface(&self, name: &str, local_addr: &str, peer_addr: &str) -> Result<()> {
        let name_c = CString::new(name).map_err(|_| Error::InvalidArg)?;
        let local_c = CString::new(local_addr).map_err(|_| Error::InvalidArg)?;
        let peer_c = CString::new(peer_addr).map_err(|_| Error::InvalidArg)?;
        map_code(unsafe {
            ffi::rrc_node_add_udp_interface(
                self.handle,
                name_c.as_ptr(),
                local_c.as_ptr(),
                peer_c.as_ptr(),
            )
        })
    }

    pub fn has_path(&self, dest_hash: &[u8]) -> Result<bool> {
        if dest_hash.len() != HASH_LEN {
            return Err(Error::InvalidArg);
        }
        let mut out = 0i32;
        map_code(unsafe {
            ffi::rrc_node_has_path(
                self.handle,
                dest_hash.as_ptr(),
                dest_hash.len(),
                &mut out,
            )
        })?;
        Ok(out != 0)
    }

    pub fn handle(&self) -> u64 {
        self.handle
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        if self.handle != 0 {
            unsafe {
                let _ = ffi::rrc_node_destroy(self.handle);
            }
            self.handle = 0;
        }
    }
}
