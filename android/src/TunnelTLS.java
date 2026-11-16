package dev.usbreverse.lab;

import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import java.math.BigInteger;
import java.net.Socket;
import java.security.KeyPairGenerator;
import java.security.KeyStore;
import java.security.MessageDigest;
import java.security.Principal;
import java.security.PrivateKey;
import java.security.cert.X509Certificate;
import java.security.spec.ECGenParameterSpec;
import java.util.Date;
import javax.net.ssl.KeyManager;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLEngine;
import javax.net.ssl.SSLParameters;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509ExtendedKeyManager;
import javax.net.ssl.X509TrustManager;
import javax.security.auth.x500.X500Principal;

/** TLS uses a non-exportable AndroidKeyStore key and an exact desktop SPKI pin. */
final class TunnelTLS {
 // The pre-release v2 alias lacked raw-digest authorization and could not
 // authenticate through Conscrypt. Preserve it; never replace saved PC trust.
 private static final String ALIAS="usb-relay-client-v2-tls";
 private static synchronized KeyStore identity()throws Exception{
  KeyStore store=KeyStore.getInstance("AndroidKeyStore");store.load(null);
  if(!store.containsAlias(ALIAS)){
   KeyPairGenerator generator=KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC,"AndroidKeyStore");
   long now=System.currentTimeMillis();
   generator.initialize(new KeyGenParameterSpec.Builder(ALIAS,KeyProperties.PURPOSE_SIGN|KeyProperties.PURPOSE_VERIFY)
    .setAlgorithmParameterSpec(new ECGenParameterSpec("secp256r1")).setDigests(KeyProperties.DIGEST_NONE,KeyProperties.DIGEST_SHA256)
    .setCertificateSubject(new X500Principal("CN=USB Relay Phone")).setCertificateSerialNumber(BigInteger.ONE)
    .setCertificateNotBefore(new Date(now-86400000L)).setCertificateNotAfter(new Date(now+315360000000L))
    .setUserAuthenticationRequired(false).build());
   generator.generateKeyPair();
  }
  return store;
 }
 private static byte[] decode(String hex){byte[] out=new byte[hex.length()/2];for(int i=0;i<out.length;i++)out[i]=(byte)Integer.parseInt(hex.substring(i*2,i*2+2),16);return out;}
 static String publicFingerprint()throws Exception{KeyStore store=identity();byte[] digest=MessageDigest.getInstance("SHA-256").digest(store.getCertificate(ALIAS).getPublicKey().getEncoded());StringBuilder text=new StringBuilder();for(byte value:digest)text.append(String.format(java.util.Locale.ROOT,"%02x",value&255));return text.toString();}
 // TLS hashes its transcript before asking Keystore to sign the digest.
 // Exercise that exact non-exportable-key operation with synthetic data.
 static boolean signingSelfTest(){try{
  KeyStore store=identity();PrivateKey key=(PrivateKey)store.getKey(ALIAS,null);
  byte[] digest=MessageDigest.getInstance("SHA-256").digest("USB Relay synthetic TLS signing probe".getBytes(java.nio.charset.StandardCharsets.US_ASCII));
  java.security.Signature signer=java.security.Signature.getInstance("NONEwithECDSA");signer.initSign(key);signer.update(digest);byte[] signature=signer.sign();
  java.security.Signature verifier=java.security.Signature.getInstance("NONEwithECDSA");verifier.initVerify(store.getCertificate(ALIAS));verifier.update(digest);return verifier.verify(signature);
 }catch(Exception error){return false;}}
 static SSLSocket connect(Socket raw,ConnectionConfig config)throws Exception{
  KeyStore store=identity();PrivateKey key=(PrivateKey)store.getKey(ALIAS,null);X509Certificate cert=(X509Certificate)store.getCertificate(ALIAS);
  X509ExtendedKeyManager manager=new X509ExtendedKeyManager(){
   public String[] getClientAliases(String type,Principal[] issuers){return "EC".equals(type)?new String[]{ALIAS}:null;}
   public String chooseClientAlias(String[] types,Principal[] issuers,Socket socket){for(String type:types)if("EC".equals(type))return ALIAS;return null;}
   public String chooseEngineClientAlias(String[] types,Principal[] issuers,SSLEngine engine){return chooseClientAlias(types,issuers,null);}
   public String[] getServerAliases(String type,Principal[] issuers){return null;}
   public String chooseServerAlias(String type,Principal[] issuers,Socket socket){return null;}
   public X509Certificate[] getCertificateChain(String alias){return ALIAS.equals(alias)?new X509Certificate[]{cert}:null;}
   public PrivateKey getPrivateKey(String alias){return ALIAS.equals(alias)?key:null;}
  };
  byte[] expected=decode(config.pin);
  X509TrustManager trust=new X509TrustManager(){
   public void checkClientTrusted(X509Certificate[] chain,String type)throws java.security.cert.CertificateException{throw new java.security.cert.CertificateException("client trust is not supported");}
   public void checkServerTrusted(X509Certificate[] chain,String type)throws java.security.cert.CertificateException{
    try{if(chain.length!=1)throw new Exception("unexpected chain");chain[0].checkValidity();byte[] actual=MessageDigest.getInstance("SHA-256").digest(chain[0].getPublicKey().getEncoded());if(!MessageDigest.isEqual(expected,actual))throw new Exception("desktop identity mismatch");}
    catch(Exception error){throw new java.security.cert.CertificateException("desktop identity verification failed");}
   }
   public X509Certificate[] getAcceptedIssuers(){return new X509Certificate[0];}
  };
  SSLContext context=SSLContext.getInstance("TLSv1.3");context.init(new KeyManager[]{manager},new TrustManager[]{trust},null);
  SSLSocket socket=(SSLSocket)context.getSocketFactory().createSocket(raw,"usb-relay.local",config.port,true);
  SSLParameters parameters=socket.getSSLParameters();parameters.setProtocols(new String[]{"TLSv1.3"});parameters.setApplicationProtocols(new String[]{"usb-relay/2"});socket.setSSLParameters(parameters);socket.startHandshake();
  if(!"usb-relay/2".equals(socket.getApplicationProtocol()))throw new java.security.cert.CertificateException("unsupported tunnel protocol");
  return socket;
 }
}
