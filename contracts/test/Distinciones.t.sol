// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {IERC721Metadata} from "@openzeppelin/contracts/token/ERC721/extensions/IERC721Metadata.sol";
import {IERC165} from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import {IERC721Errors} from "@openzeppelin/contracts/interfaces/draft-IERC6093.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Base64} from "@openzeppelin/contracts/utils/Base64.sol";

import {Base} from "./Base.t.sol";
import {Distinciones} from "../src/Distinciones.sol";
import {IERC5192} from "../src/IERC5192.sol";

contract DistincionesTest is Base {
    // ------------------------------------------------------------- despliegue

    function test_DespliegueCargaElCatalogo() public view {
        assertEq(distinciones.cantidadPiezas(), CANTIDAD_PIEZAS);
        assertEq(distinciones.firmante(), firmante);
        assertEq(distinciones.owner(), duenio);

        (string memory nombre, uint8 obra) = distinciones.piezas(34);
        assertEq(nombre, "Obra 0");
        assertEq(obra, 0);
    }

    function test_DespliegueSinFirmanteRevierte() public {
        vm.expectRevert(Distinciones.FirmanteVacio.selector);
        new Distinciones(duenio, address(0), _catalogo());
    }

    function test_DespliegueSinPiezasRevierte() public {
        vm.expectRevert(Distinciones.SinPiezas.selector);
        new Distinciones(duenio, firmante, new Distinciones.Pieza[](0));
    }

    // ------------------------------------------------------------ mint feliz

    function test_MintFeliz() public {
        uint16 pieza = 7;
        uint64 deadline = _deadline();
        uint256 id = distinciones.idDe(titular, pieza);

        vm.expectEmit(true, true, true, true, address(distinciones));
        emit IERC721.Transfer(address(0), titular, id);
        vm.expectEmit(true, true, true, true, address(distinciones));
        emit IERC5192.Locked(id);

        distinciones.mint(titular, pieza, deadline, _firmar(PK_FIRMANTE, titular, pieza, deadline));

        assertEq(distinciones.ownerOf(id), titular);
        assertEq(distinciones.balanceOf(titular), 1);
    }

    function test_ElIdEsDeterministaYGuardaLaPieza() public view {
        uint16 pieza = 4242 % CANTIDAD_PIEZAS;
        uint256 id = distinciones.idDe(titular, pieza);

        assertEq(id, (uint256(uint160(titular)) << 16) | pieza);
        assertEq(uint16(id), pieza);
        assertEq(address(uint160(id >> 16)), titular);
    }

    /// @dev Adelantarse en el mempool no sirve de nada: el voucher dice `to`.
    function test_CualquieraPuedeMandarLaTransaccion() public {
        uint16 pieza = 1;
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, pieza, deadline);

        vm.prank(makeAddr("entrometido"));
        distinciones.mint(titular, pieza, deadline, firma);

        assertEq(distinciones.ownerOf(distinciones.idDe(titular, pieza)), titular);
    }

    // ------------------------------------------------------------ mint feo

    function test_FirmaInvalidaRevierte() public {
        uint64 deadline = _deadline();
        bytes memory basura = new bytes(65);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(titular, 0, deadline, basura);
    }

    function test_FirmaDeOtroFirmanteRevierte() public {
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_INTRUSO, titular, 0, deadline);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(titular, 0, deadline, firma);
    }

    function test_FirmaDeOtraPiezaRevierte() public {
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, 3, deadline);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(titular, 4, deadline, firma);
    }

    function test_FirmaParaOtraDireccionRevierte() public {
        uint64 deadline = _deadline();
        address otro = makeAddr("otro");
        bytes memory firma = _firmar(PK_FIRMANTE, otro, 0, deadline);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(titular, 0, deadline, firma);
    }

    /// @dev El dominio ata la firma a este contrato: la misma firma en otro
    ///      despliegue no sirve, que es justo lo que el §8 promete.
    function test_FirmaDeOtroContratoRevierte() public {
        uint16 pieza = 2;
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, pieza, deadline);

        Distinciones gemelo = new Distinciones(duenio, firmante, _catalogo());

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        gemelo.mint(titular, pieza, deadline, firma);
    }

    function test_VoucherVencidoRevierte() public {
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, 0, deadline);

        vm.warp(uint256(deadline) + 1);

        vm.expectRevert(Distinciones.Vencido.selector);
        distinciones.mint(titular, 0, deadline, firma);
    }

    function test_JustoEnElDeadlineTodaviaVale() public {
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, 0, deadline);

        vm.warp(deadline);
        distinciones.mint(titular, 0, deadline, firma);

        assertEq(distinciones.balanceOf(titular), 1);
    }

    function test_PiezaFueraDelCatalogoRevierte() public {
        uint16 pieza = CANTIDAD_PIEZAS;
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, pieza, deadline);

        vm.expectRevert(abi.encodeWithSelector(Distinciones.PiezaInexistente.selector, pieza));
        distinciones.mint(titular, pieza, deadline, firma);
    }

    /// @dev El id determinista es toda la protección contra repetición: el
    ///      segundo `_mint` choca con un id que ya existe.
    function test_ReclamarDosVecesRevierte() public {
        uint16 pieza = 5;
        uint64 deadline = _deadline();
        bytes memory firma = _firmar(PK_FIRMANTE, titular, pieza, deadline);

        distinciones.mint(titular, pieza, deadline, firma);

        uint256 id = distinciones.idDe(titular, pieza);
        vm.expectRevert(abi.encodeWithSelector(Distinciones.YaReclamada.selector, id));
        distinciones.mint(titular, pieza, deadline, firma);

        assertEq(distinciones.balanceOf(titular), 1);
    }

    function test_MismaPiezaParaOtraDireccionSeAcunia() public {
        uint16 pieza = 5;
        uint64 deadline = _deadline();
        address otro = makeAddr("otro");

        distinciones.mint(titular, pieza, deadline, _firmar(PK_FIRMANTE, titular, pieza, deadline));
        distinciones.mint(otro, pieza, deadline, _firmar(PK_FIRMANTE, otro, pieza, deadline));

        assertEq(distinciones.ownerOf(distinciones.idDe(titular, pieza)), titular);
        assertEq(distinciones.ownerOf(distinciones.idDe(otro, pieza)), otro);
    }

    // ------------------------------------------------------------- soulbound

    function test_LockedDaVerdaderoParaUnTokenAcunado() public {
        assertTrue(distinciones.locked(_acunar(titular, 3)));
    }

    /// El ERC-5192 pide que preguntar por un token que no existe reverta.
    function test_LockedDeUnTokenInexistenteRevierte() public {
        vm.expectRevert(abi.encodeWithSignature("ERC721NonexistentToken(uint256)", 0));
        distinciones.locked(0);
    }

    function test_TransferFromRevierte() public {
        uint256 id = _acunar(titular, 9);
        address otro = makeAddr("otro");

        vm.prank(titular);
        vm.expectRevert(Distinciones.NoSeTransfiere.selector);
        distinciones.transferFrom(titular, otro, id);
    }

    function test_SafeTransferFromRevierte() public {
        uint256 id = _acunar(titular, 9);
        address otro = makeAddr("otro");

        vm.prank(titular);
        vm.expectRevert(Distinciones.NoSeTransfiere.selector);
        distinciones.safeTransferFrom(titular, otro, id);
    }

    /// @dev Aprobar se puede —no cuesta nada dejarlo—, pero de nada sirve.
    function test_AprobarNoAlcanzaParaMover() public {
        uint256 id = _acunar(titular, 9);
        address operador = makeAddr("operador");

        vm.prank(titular);
        distinciones.approve(operador, id);

        vm.prank(operador);
        vm.expectRevert(Distinciones.NoSeTransfiere.selector);
        distinciones.transferFrom(titular, operador, id);
    }

    // ------------------------------------------------------------- metadatos

    function test_TokenURIEsUnJSONEnBase64() public {
        uint16 pieza = 34; // la primera obra
        uint256 id = _acunar(titular, pieza);

        string memory esperado = string.concat(
            "data:application/json;base64,",
            Base64.encode(
                bytes(
                    '{"name":"Obra 0","description":"Distincion de Lost in Translation. Obra 0. No se transfiere: es de quien la gano."}'
                )
            )
        );

        assertEq(distinciones.tokenURI(id), esperado);
    }

    function test_TokenURIDeTokenInexistenteRevierte() public {
        uint256 id = distinciones.idDe(titular, 0);

        vm.expectRevert(abi.encodeWithSelector(IERC721Errors.ERC721NonexistentToken.selector, id));
        distinciones.tokenURI(id);
    }

    function test_SupportsInterface() public view {
        assertTrue(distinciones.supportsInterface(type(IERC5192).interfaceId), "5192");
        assertEq(type(IERC5192).interfaceId, bytes4(0xb45a3c0e), "el id del 5192 cambio");

        assertTrue(distinciones.supportsInterface(type(IERC721).interfaceId), "721");
        assertTrue(distinciones.supportsInterface(type(IERC721Metadata).interfaceId), "metadata");
        assertTrue(distinciones.supportsInterface(type(IERC165).interfaceId), "165");

        assertFalse(distinciones.supportsInterface(bytes4(0xdeadbeef)), "cualquier cosa");
    }

    // -------------------------------------------------------------- rotación

    function test_SetFirmanteSoloElDuenio() public {
        address intruso = makeAddr("intruso");

        vm.prank(intruso);
        vm.expectRevert(
            abi.encodeWithSelector(Ownable.OwnableUnauthorizedAccount.selector, intruso)
        );
        distinciones.setFirmante(vm.addr(PK_INTRUSO));
    }

    function test_SetFirmanteEnCeroRevierte() public {
        vm.prank(duenio);
        vm.expectRevert(Distinciones.FirmanteVacio.selector);
        distinciones.setFirmante(address(0));
    }

    /// @dev Rotar es la única respuesta que tenemos si se filtra la clave: las
    ///      firmas viejas dejan de valer en el acto.
    function test_RotarInvalidaLasFirmasViejas() public {
        uint16 pieza = 11;
        uint64 deadline = _deadline();
        bytes memory firmaVieja = _firmar(PK_FIRMANTE, titular, pieza, deadline);
        address nuevo = vm.addr(PK_INTRUSO);

        vm.expectEmit(true, true, true, true, address(distinciones));
        emit Distinciones.FirmanteCambiado(firmante, nuevo);
        vm.prank(duenio);
        distinciones.setFirmante(nuevo);
        assertEq(distinciones.firmante(), nuevo);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(titular, pieza, deadline, firmaVieja);

        distinciones.mint(titular, pieza, deadline, _firmar(PK_INTRUSO, titular, pieza, deadline));
        assertEq(distinciones.balanceOf(titular), 1);
    }

    // ------------------------------------------------------------------ fuzz

    /// @notice La propiedad de fondo: con cualquier dirección, cualquier pieza y
    ///         cualquier sarta de bytes por firma, no se acuña nada.
    function testFuzz_SinFirmaValidaNoSeAcunia(
        address to,
        uint16 pieza,
        uint64 deadline,
        bytes memory firma
    ) public {
        vm.assume(to != address(0));

        vm.expectRevert();
        distinciones.mint(to, pieza, deadline, firma);

        assertEq(distinciones.balanceOf(to), 0);
    }

    /// @notice Lo mismo, pero con firmas bien armadas por una clave que no es la
    ///         del servidor: es el caso que de verdad podría colarse.
    function testFuzz_FirmaDeCualquierOtraClaveNoSeAcunia(
        address to,
        uint16 pieza,
        uint256 pkIntruso,
        uint64 deadline
    ) public {
        vm.assume(to != address(0));
        pieza = uint16(bound(pieza, 0, CANTIDAD_PIEZAS - 1));
        deadline = uint64(bound(deadline, block.timestamp, type(uint64).max));
        pkIntruso = bound(pkIntruso, 1, ORDEN_SECP256K1 - 1);
        vm.assume(pkIntruso != PK_FIRMANTE);

        bytes memory firma = _firmar(pkIntruso, to, pieza, deadline);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(to, pieza, deadline, firma);

        assertEq(distinciones.balanceOf(to), 0);
    }

    /// @notice Y el reverso: con la firma del servidor, siempre se acuña, y el id
    ///         que sale es el que el front podía calcular solo.
    function testFuzz_ConLaFirmaDelServidorSiempreSeAcunia(
        address to,
        uint16 pieza,
        uint64 deadline
    ) public {
        vm.assume(to != address(0));
        pieza = uint16(bound(pieza, 0, CANTIDAD_PIEZAS - 1));
        deadline = uint64(bound(deadline, block.timestamp, type(uint64).max));

        distinciones.mint(to, pieza, deadline, _firmar(PK_FIRMANTE, to, pieza, deadline));

        uint256 id = (uint256(uint160(to)) << 16) | pieza;
        assertEq(distinciones.ownerOf(id), to);
        assertTrue(distinciones.locked(id));
    }

    // ----------------------------------------------------------------- ayuda

    function _acunar(address to, uint16 pieza) internal returns (uint256 id) {
        uint64 deadline = _deadline();
        distinciones.mint(to, pieza, deadline, _firmar(PK_FIRMANTE, to, pieza, deadline));
        return distinciones.idDe(to, pieza);
    }
}
